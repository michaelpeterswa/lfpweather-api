package timescale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"
	_ "time/tzdata" // embed the zoneinfo database so America/Los_Angeles loads on a minimal image

	"github.com/cespare/xxhash/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// recordsLocation is the station's civil timezone. Period boundaries (a day, a
// week, a month, a year) are civil boundaries at the station, not UTC.
var recordsLocation = mustLoadLocation("America/Los_Angeles")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load location %q: %v", name, err))
	}
	return loc
}

// recordsAllStart bounds the "all" period on the low side. The station has no
// data before this instant, so it is a safe lower bound that never excludes a
// real reading.
var recordsAllStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// recordDef is one row of the records table: a metric plus whether a record low
// is meaningful for it. Every metric has a record high; only some (temperature,
// humidity, pressure) have a meaningful record low.
type recordDef struct {
	Metric string
	Low    bool
}

// recordDefs is the curated, ordered set of metrics shown on the records page.
// Each metric name resolves through the same allowlist and column catalog as
// the structured query endpoint, so the set can only ever name a known column.
var recordDefs = []recordDef{
	{"temperature", true},
	{"humidity", true},
	{"pressure", true},
	{"wind_gust", false},
	{"rain_rate", false},
	{"rain_24h", false},
	{"solar_radiation", false},
	{"uv_index", false},
	{"erc", false},
	{"burning_index", false},
}

// RecordExtreme is a single extreme reading and the time it happened.
type RecordExtreme struct {
	Value float64   `json:"value"`
	Time  time.Time `json:"time"`
}

// MetricRecord is the record high and (where meaningful) record low of one
// metric over the period. A field is absent when the window holds no reading.
type MetricRecord struct {
	Metric string         `json:"metric"`
	High   *RecordExtreme `json:"high,omitempty"`
	Low    *RecordExtreme `json:"low,omitempty"`
}

// RecordsResponse is the payload of GET /api/v1/records/{period}. Complete is
// true when the period has ended and its records can no longer change.
type RecordsResponse struct {
	Period   string         `json:"period"`
	Start    time.Time      `json:"start"`
	End      time.Time      `json:"end"`
	Complete bool           `json:"complete"`
	Records  []MetricRecord `json:"records"`
}

// resolvePeriod turns a period name and an anchor time into an absolute
// [start, end) window at the station's civil timezone. complete is true when
// the window has fully elapsed. The "all" period runs from the station's
// earliest possible data to now and is never complete.
func resolvePeriod(period string, at, now time.Time) (start, end time.Time, complete bool, err error) {
	if period == "all" {
		return recordsAllStart, now, false, nil
	}

	l := at.In(recordsLocation)
	y, m, d := l.Date()

	switch period {
	case "day":
		start = time.Date(y, m, d, 0, 0, 0, 0, recordsLocation)
		end = start.AddDate(0, 0, 1)
	case "week":
		// Week starts on Monday, matching Postgres date_trunc('week').
		offset := (int(l.Weekday()) + 6) % 7
		start = time.Date(y, m, d, 0, 0, 0, 0, recordsLocation).AddDate(0, 0, -offset)
		end = start.AddDate(0, 0, 7)
	case "month":
		start = time.Date(y, m, 1, 0, 0, 0, 0, recordsLocation)
		end = start.AddDate(0, 1, 0)
	case "year":
		start = time.Date(y, 1, 1, 0, 0, 0, 0, recordsLocation)
		end = start.AddDate(1, 0, 0)
	default:
		return time.Time{}, time.Time{}, false, verr("unknown period %q (allowed: day, week, month, year, all)", period)
	}

	return start, end, !end.After(now), nil
}

// ParseAt parses the optional ?at= query parameter. It accepts an RFC3339
// timestamp or a plain YYYY-MM-DD date, which is read at the station timezone.
func ParseAt(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, recordsLocation); err == nil {
		return t, nil
	}
	return time.Time{}, verr("invalid at %q; use an RFC3339 timestamp or a YYYY-MM-DD date", s)
}

//go:embed queries/getrecord.pgsql.gotmpl
var getRecordTemplate string

var getRecordTmpl = template.Must(template.New("getRecord").Parse(getRecordTemplate))

type recordTemplateParams struct {
	Table          string
	Column         string
	SerialFiltered bool
	Serial         string
	ExcludeSpinup  bool
	Direction      string // DESC for a high, ASC for a low
}

// GetRecords returns the curated record set for a period, cache first. A
// completed period is cached with the longer tier; an in-progress period uses
// the shorter tier so it stays fresh.
func (c *TimescaleClient) GetRecords(ctx context.Context, catalog Catalog, period string, at, now time.Time) (*RecordsResponse, error) {
	if at.After(now) {
		at = now
	}

	start, end, complete, err := resolvePeriod(period, at, now)
	if err != nil {
		return nil, err
	}

	cacheKey := c.recordsCacheKey(period, start)
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, cacheKey).Result()
		if err == nil {
			var cached RecordsResponse
			if err := json.Unmarshal([]byte(res), &cached); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &cached, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	resp := &RecordsResponse{
		Period:   period,
		Start:    start,
		End:      end,
		Complete: complete,
	}

	for _, rd := range recordDefs {
		def, err := resolveMetric(rd.Metric, catalog)
		if err != nil {
			// A record metric names a column outside the live catalog. Skip it
			// rather than fail the whole page; the column may not exist yet.
			slog.Error("skipping unresolved record metric", slog.String("metric", rd.Metric), slog.String("error", err.Error()))
			continue
		}

		mr := MetricRecord{Metric: rd.Metric}
		mr.High, err = c.queryRecord(ctx, def, start, end, "DESC")
		if err != nil {
			return nil, err
		}
		if rd.Low {
			mr.Low, err = c.queryRecord(ctx, def, start, end, "ASC")
			if err != nil {
				return nil, err
			}
		}
		resp.Records = append(resp.Records, mr)
	}

	if c.Dfly != nil {
		ttl := c.recordsCacheCurrent
		if complete {
			ttl = c.recordsCacheComplete
		}
		respJSON, err := json.Marshal(resp)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else if err := c.Dfly.GetClient().Set(ctx, cacheKey, respJSON, ttl).Err(); err != nil {
			slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
		}
	}

	return resp, nil
}

// recordsCacheKey keys a record set by period and window start. The window
// start alone fixes the window (period plus start implies end), so a completed
// period keeps a stable key forever, while the current period's key is stable
// within its short cache tier.
func (c *TimescaleClient) recordsCacheKey(period string, start time.Time) string {
	raw := strings.Join([]string{"records", period, start.UTC().Format(time.RFC3339)}, "|")
	return fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, strconv.FormatUint(xxhash.Sum64String(raw), 16))
}

// queryRecord runs one extreme lookup and returns the reading, or nil when the
// window holds no reading for the metric.
func (c *TimescaleClient) queryRecord(ctx context.Context, def metricDef, start, end time.Time, direction string) (*RecordExtreme, error) {
	params := recordTemplateParams{
		Table:          def.Table,
		Column:         def.Column,
		SerialFiltered: def.SerialFiltered,
		Serial:         airgradientSerial,
		// fire_danger carries late-2024 spin-up warm-up rows that would report a
		// false extreme; exclude them, matching the fire danger summary.
		ExcludeSpinup: def.Table == "fire_danger",
		Direction:     direction,
	}

	query := bytes.NewBuffer(nil)
	if err := getRecordTmpl.Execute(query, params); err != nil {
		return nil, fmt.Errorf("failed to execute record template: %w", err)
	}

	var rec RecordExtreme
	err := c.Pool.QueryRow(ctx, query.String(), start, end).Scan(&rec.Time, &rec.Value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query %s.%s record: %w", def.Table, def.Column, err)
	}
	return &rec, nil
}
