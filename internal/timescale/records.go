package timescale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"
	_ "time/tzdata" // embed the zoneinfo database so America/Los_Angeles loads on a minimal image

	"github.com/cespare/xxhash/v2"
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
	// Max, when set, is the highest physically plausible reading. A record high
	// ignores anything above it, so a single glitched sample cannot stand as a
	// record forever. It bounds only highs: the glitches seen are all spikes.
	Max float64
}

// recordDefs is the curated, ordered set of metrics shown on the records page.
// Each metric name resolves through the same allowlist and column catalog as
// the structured query endpoint, so the set can only ever name a known column.
//
// The Max bounds come from glitches that became all-time records once the
// all-time period started loading. Each was a single sample with ordinary
// readings either side and no matching change in any other measurement:
//
//   - wind_gust 175.25 mph (2026-04-15); the next-highest day in two years
//     was 17.9 mph, and the hours either side read about 5 mph.
//   - rain_rate 3032 counts, 30.32 in/hr (2025-11-14), and 691.5 counts
//     (2025-03-20), with the daily rain total flat through both. The highest
//     real rate is 229 counts (2.29 in/hr). Rain is in 0.01 in bucket tips.
//
// The bounds sit far above anything real at this station.
var recordDefs = []recordDef{
	{Metric: "temperature", Low: true},
	{Metric: "humidity", Low: true},
	{Metric: "pressure", Low: true},
	{Metric: "wind_gust", Max: 100},
	{Metric: "rain_rate", Max: 500},
	{Metric: "rain_24h"},
	{Metric: "solar_radiation"},
	{Metric: "uv_index"},
	{Metric: "erc"},
	{Metric: "burning_index"},
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

//go:embed queries/getrecordvalues.pgsql.gotmpl
var getRecordValuesTemplate string

//go:embed queries/getrecordtimes.pgsql.gotmpl
var getRecordTimesTemplate string

var (
	getRecordValuesTmpl = template.Must(template.New("getRecordValues").Parse(getRecordValuesTemplate))
	getRecordTimesTmpl  = template.Must(template.New("getRecordTimes").Parse(getRecordTimesTemplate))
)

// recordExtreme is one aggregate in a records scan: the high (max) or low
// (min) of a column, and where its result goes.
type recordExtreme struct {
	Column string
	Agg    string  // "max" for a high, "min" for a low
	Max    float64 // when set, readings above it are ignored (see recordDef)
	Param  int     // placeholder holding the value in the times query
	record int     // index into RecordsResponse.Records
	high   bool
}

// recordScan is every extreme the curated set needs from one table. All of a
// table's extremes are found in two scans of the window (values, then times)
// instead of one ORDER BY ... LIMIT 1 scan per extreme: the whole-window scan
// is the expensive part, and vantagepro2plus alone used to be scanned 11 times.
type recordScan struct {
	Table          string
	SerialFiltered bool
	Serial         string
	ExcludeSpinup  bool
	Extremes       []recordExtreme
}

// buildRecordScans resolves the curated set against the catalog and groups its
// extremes by table, in recordDefs order. It returns the (still empty) records
// the scans fill in.
func buildRecordScans(catalog Catalog) ([]MetricRecord, []*recordScan) {
	var records []MetricRecord
	var scans []*recordScan
	byTable := map[string]*recordScan{}

	for _, rd := range recordDefs {
		def, err := resolveMetric(rd.Metric, catalog)
		if err != nil {
			// A record metric names a column outside the live catalog. Skip it
			// rather than fail the whole page; the column may not exist yet.
			slog.Error("skipping unresolved record metric", slog.String("metric", rd.Metric), slog.String("error", err.Error()))
			continue
		}

		scan, ok := byTable[def.Table]
		if !ok {
			scan = &recordScan{
				Table:          def.Table,
				SerialFiltered: def.SerialFiltered,
				Serial:         airgradientSerial,
				// fire_danger carries late-2024 spin-up warm-up rows that would
				// report a false extreme; exclude them, matching the fire danger
				// summary.
				ExcludeSpinup: def.Table == "fire_danger",
			}
			byTable[def.Table] = scan
			scans = append(scans, scan)
		}

		idx := len(records)
		records = append(records, MetricRecord{Metric: rd.Metric})
		scan.Extremes = append(scan.Extremes, recordExtreme{Column: def.Column, Agg: "max", Max: rd.Max, record: idx, high: true})
		if rd.Low {
			scan.Extremes = append(scan.Extremes, recordExtreme{Column: def.Column, Agg: "min", record: idx})
		}
	}

	return records, scans
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

	if period == "all" {
		resp.Records, err = c.composeAllTime(ctx, catalog, now)
	} else {
		resp.Records, err = c.queryRecords(ctx, catalog, start, end)
	}
	if err != nil {
		return nil, err
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

// composeAllTime builds the all-time records from the records of each calendar
// year. Scanning the whole history in one request outgrew the query timeout,
// and it never recovered because a failed result is never cached. A completed
// year is cached with the long tier, so after the first fill an all-time
// request costs about as much as the current year's. Each year is cached as it
// completes, so a cold fill that times out still makes progress for the next
// request.
func (c *TimescaleClient) composeAllTime(ctx context.Context, catalog Catalog, now time.Time) ([]MetricRecord, error) {
	var years []*RecordsResponse
	for y := recordsAllStart.Year(); y <= now.In(recordsLocation).Year(); y++ {
		yr, err := c.GetRecords(ctx, catalog, "year", time.Date(y, 1, 1, 0, 0, 0, 0, recordsLocation), now)
		if err != nil {
			return nil, err
		}
		years = append(years, yr)
	}
	return mergeRecords(years), nil
}

// mergeRecords combines record sets of consecutive periods, given oldest
// first, into the records of their union. A tie keeps the earlier reading, so
// a record is credited to when it was first reached.
func mergeRecords(periods []*RecordsResponse) []MetricRecord {
	var merged []MetricRecord
	index := map[string]int{}

	for _, p := range periods {
		for _, mr := range p.Records {
			i, ok := index[mr.Metric]
			if !ok {
				index[mr.Metric] = len(merged)
				merged = append(merged, MetricRecord{Metric: mr.Metric})
				i = len(merged) - 1
			}
			acc := &merged[i]
			if mr.High != nil && (acc.High == nil || mr.High.Value > acc.High.Value) {
				acc.High = mr.High
			}
			if mr.Low != nil && (acc.Low == nil || mr.Low.Value < acc.Low.Value) {
				acc.Low = mr.Low
			}
		}
	}

	// Present metrics in curated order, whatever order the periods listed them.
	order := map[string]int{}
	for i, rd := range recordDefs {
		order[rd.Metric] = i
	}
	sort.SliceStable(merged, func(a, b int) bool {
		return order[merged[a].Metric] < order[merged[b].Metric]
	})

	return merged
}

// recordsCacheVersion is part of every records cache key. Bump it when the
// record definitions change, so completed periods cached under the old ones
// (for the long tier, a month) are recomputed instead of served.
const recordsCacheVersion = "2"

// recordsCacheKey keys a record set by period and window start. The window
// start alone fixes the window (period plus start implies end), so a completed
// period keeps a stable key forever, while the current period's key is stable
// within its short cache tier.
func (c *TimescaleClient) recordsCacheKey(period string, start time.Time) string {
	raw := strings.Join([]string{"records", recordsCacheVersion, period, start.UTC().Format(time.RFC3339)}, "|")
	return fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, strconv.FormatUint(xxhash.Sum64String(raw), 16))
}

// queryRecords finds the curated record set over the window [start, end). A
// metric with no reading in the window keeps a nil High/Low.
func (c *TimescaleClient) queryRecords(ctx context.Context, catalog Catalog, start, end time.Time) ([]MetricRecord, error) {
	records, scans := buildRecordScans(catalog)
	for _, scan := range scans {
		if err := c.runRecordScan(ctx, scan, start, end, records); err != nil {
			return nil, err
		}
	}
	return records, nil
}

// runRecordScan finds every extreme of one table in two scans: the first takes
// the plain max/min of each column, the second the earliest time each value was
// seen. Plain aggregates keep the per-row work cheap; an argmax in one scan
// (max over ARRAY[value, time]) measured about five times slower than the
// separate queries it replaced.
func (c *TimescaleClient) runRecordScan(ctx context.Context, scan *recordScan, start, end time.Time, records []MetricRecord) error {
	query := bytes.NewBuffer(nil)
	if err := getRecordValuesTmpl.Execute(query, scan); err != nil {
		return fmt.Errorf("failed to execute record values template: %w", err)
	}

	values := make([]*float64, len(scan.Extremes))
	dest := make([]any, len(values))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := c.Pool.QueryRow(ctx, query.String(), start, end).Scan(dest...); err != nil {
		return fmt.Errorf("failed to query %s record values: %w", scan.Table, err)
	}

	// Only extremes with a reading need a time; a nil value means the window
	// holds no reading for that column.
	found := &recordScan{
		Table:          scan.Table,
		SerialFiltered: scan.SerialFiltered,
		Serial:         scan.Serial,
		ExcludeSpinup:  scan.ExcludeSpinup,
	}
	args := []any{start, end}
	for i, ex := range scan.Extremes {
		if values[i] == nil {
			continue
		}
		args = append(args, *values[i])
		ex.Param = len(args)
		found.Extremes = append(found.Extremes, ex)
	}
	if len(found.Extremes) == 0 {
		return nil
	}

	query.Reset()
	if err := getRecordTimesTmpl.Execute(query, found); err != nil {
		return fmt.Errorf("failed to execute record times template: %w", err)
	}

	times := make([]*time.Time, len(found.Extremes))
	dest = make([]any, len(times))
	for i := range times {
		dest[i] = &times[i]
	}
	if err := c.Pool.QueryRow(ctx, query.String(), args...).Scan(dest...); err != nil {
		return fmt.Errorf("failed to query %s record times: %w", scan.Table, err)
	}

	for i, ex := range found.Extremes {
		if times[i] == nil {
			continue
		}
		rec := &RecordExtreme{Value: args[ex.Param-1].(float64), Time: *times[i]}
		if ex.high {
			records[ex.record].High = rec
		} else {
			records[ex.record].Low = rec
		}
	}
	return nil
}
