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

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"
)

// MetricType describes how a metric is aggregated over a time bucket.
type MetricType string

const (
	// MetricTypeGauge is a numeric sensor reading summarized with avg/min/max/first/last.
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeCount is an event stream summarized by counting rows per bucket.
	MetricTypeCount MetricType = "count"
)

// airgradientSerial is the single AirGradient unit whose readings are exposed.
const airgradientSerial = "84fce6070dd4"

// metricDef maps a public metric name onto a concrete source relation + column.
// Table is the source relation under the sensors schema; swapping it for a
// TimescaleDB continuous aggregate later requires no change to the API contract.
type metricDef struct {
	Table          string
	Column         string // empty for count metrics
	Type           MetricType
	SerialFiltered bool
}

// metricRegistry is the allowlist. Only names present here can be queried, and
// every Table/Column value is a server-controlled constant that reaches SQL via
// text/template — no caller input is ever interpolated into the query text.
var metricRegistry = map[string]metricDef{
	// vantagepro2plus weather station
	"temperature":     {Table: "vantagepro2plus", Column: "temperature", Type: MetricTypeGauge},
	"humidity":        {Table: "vantagepro2plus", Column: "humidity", Type: MetricTypeGauge},
	"pressure":        {Table: "vantagepro2plus", Column: "barometer_sea_level", Type: MetricTypeGauge},
	"solar_radiation": {Table: "vantagepro2plus", Column: "solar_radiation", Type: MetricTypeGauge},
	"wind_speed":      {Table: "vantagepro2plus", Column: "wind_speed_last", Type: MetricTypeGauge},
	"wind_gust":       {Table: "vantagepro2plus", Column: "wind_speed_high_last_10_min", Type: MetricTypeGauge},
	"rain_rate":       {Table: "vantagepro2plus", Column: "rain_rate_last", Type: MetricTypeGauge},
	"rain_24h":        {Table: "vantagepro2plus", Column: "rain_last_24_hour", Type: MetricTypeGauge},
	"uv_index":        {Table: "vantagepro2plus", Column: "uv_index", Type: MetricTypeGauge},
	// airgradient air quality
	"aqi":        {Table: "airgradient_aqi", Column: "aqi", Type: MetricTypeGauge, SerialFiltered: true},
	"co2":        {Table: "airgradient", Column: "rco2", Type: MetricTypeGauge, SerialFiltered: true},
	"nox_index":  {Table: "airgradient", Column: "nox_index", Type: MetricTypeGauge, SerialFiltered: true},
	"tvoc_index": {Table: "airgradient", Column: "tvoc_index", Type: MetricTypeGauge, SerialFiltered: true},
	// birdnet acoustic detections
	"birdnet": {Table: "birdnet", Type: MetricTypeCount},
}

var (
	allowedGaugeAggs = map[string]bool{
		"avg": true, "min": true, "max": true, "count": true, "first": true, "last": true,
	}
	defaultGaugeAggs = []string{"avg", "min", "max"}
)

// bucketDef is one rung of the adaptive bucket ladder.
type bucketDef struct {
	Name     string        // caller-facing name, e.g. "5m"
	Interval string        // postgres interval literal, e.g. "5 minutes"
	Duration time.Duration // used for adaptive selection
}

// bucketLadder is ordered finest-to-coarsest. Buckets grow with the observed
// span so the point count stays bounded (see pickBucket).
var bucketLadder = []bucketDef{
	{"5m", "5 minutes", 5 * time.Minute},
	{"15m", "15 minutes", 15 * time.Minute},
	{"30m", "30 minutes", 30 * time.Minute},
	{"1h", "1 hour", time.Hour},
	{"3h", "3 hours", 3 * time.Hour},
	{"6h", "6 hours", 6 * time.Hour},
	{"12h", "12 hours", 12 * time.Hour},
	{"1d", "1 day", 24 * time.Hour},
	{"1w", "1 week", 7 * 24 * time.Hour},
}

func lookupBucket(name string) (bucketDef, bool) {
	for _, b := range bucketLadder {
		if b.Name == name {
			return b, true
		}
	}
	return bucketDef{}, false
}

func bucketNames() string {
	names := make([]string, len(bucketLadder))
	for i, b := range bucketLadder {
		names[i] = b.Name
	}
	return strings.Join(names, ", ")
}

// pickBucket returns the finest bucket that keeps the series at or under
// targetPoints, so longer ranges automatically roll up into coarser buckets.
func pickBucket(span time.Duration, targetPoints int) bucketDef {
	for _, b := range bucketLadder {
		if int(span/b.Duration) <= targetPoints {
			return b
		}
	}
	return bucketLadder[len(bucketLadder)-1]
}

// resolveRange turns a named range into an absolute [start, end) window.
func resolveRange(name string, now time.Time, maxRange time.Duration) (time.Time, time.Time, bool) {
	switch name {
	case "12h":
		return now.Add(-12 * time.Hour), now, true
	case "24h":
		return now.Add(-24 * time.Hour), now, true
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now, true
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now, true
	case "90d":
		return now.Add(-90 * 24 * time.Hour), now, true
	case "ytd":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()), now, true
	case "last_12mo":
		return now.AddDate(-1, 0, 0), now, true
	case "prev_year":
		start := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		end := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return start, end, true
	case "all":
		return now.Add(-maxRange), now, true
	default:
		return time.Time{}, time.Time{}, false
	}
}

// QueryRequest is the JSON body accepted by POST /api/v1/query.
type QueryRequest struct {
	Metric       string   `json:"metric"`
	Range        string   `json:"range,omitempty"`  // named range; mutually exclusive with start/end
	Start        *string  `json:"start,omitempty"`  // RFC3339
	End          *string  `json:"end,omitempty"`    // RFC3339 (defaults to now)
	Bucket       string   `json:"bucket,omitempty"` // override; empty = auto by span
	Aggregations []string `json:"aggregations,omitempty"`
	GroupBy      string   `json:"group_by,omitempty"` // count metrics only; "common_name"
}

// QueryLimits are the guardrails applied when planning a query.
type QueryLimits struct {
	MaxRange     time.Duration
	TargetPoints int
	MaxPoints    int
}

// QueryPlan is a validated, ready-to-execute query. All fields are unexported;
// callers treat it as an opaque handle produced by PlanQuery and consumed by RunQuery.
type QueryPlan struct {
	metric   string
	def      metricDef
	start    time.Time
	end      time.Time
	bucket   bucketDef
	aggs     []string
	groupBy  bool
	cacheKey string
}

// QueryPoint is one bucket of a series. Unset aggregations are omitted.
type QueryPoint struct {
	Time       time.Time `json:"time"`
	Avg        *float64  `json:"avg,omitempty"`
	Min        *float64  `json:"min,omitempty"`
	Max        *float64  `json:"max,omitempty"`
	First      *float64  `json:"first,omitempty"`
	Last       *float64  `json:"last,omitempty"`
	Count      *int64    `json:"count,omitempty"`
	CommonName string    `json:"common_name,omitempty"`
}

// QueryResponse wraps the series with the resolved window and bucket so the
// caller knows exactly what resolution it received (buckets are auto-selected).
type QueryResponse struct {
	Metric  string       `json:"metric"`
	Type    MetricType   `json:"type"`
	Start   time.Time    `json:"start"`
	End     time.Time    `json:"end"`
	Bucket  string       `json:"bucket"`
	GroupBy string       `json:"group_by,omitempty"`
	Points  []QueryPoint `json:"points"`
}

// QueryValidationError marks a caller mistake (bad input) so the handler can
// return 400 rather than 500.
type QueryValidationError struct{ Msg string }

func (e *QueryValidationError) Error() string { return e.Msg }

func verr(format string, a ...any) error {
	return &QueryValidationError{Msg: fmt.Sprintf(format, a...)}
}

// PlanQuery validates a request against the allowlists and limits and resolves
// the concrete window + bucket. It performs no I/O and is safe to unit test.
func PlanQuery(req QueryRequest, limits QueryLimits, now time.Time) (*QueryPlan, error) {
	def, ok := metricRegistry[req.Metric]
	if !ok {
		return nil, verr("unknown metric %q", req.Metric)
	}

	start, end, rangeKey, err := resolveWindow(req, limits, now)
	if err != nil {
		return nil, err
	}

	if end.After(now) {
		end = now
	}
	if !start.Before(end) {
		return nil, verr("start must be before end")
	}
	span := end.Sub(start)
	if span > limits.MaxRange {
		return nil, verr("requested range %s exceeds maximum %s", span, limits.MaxRange)
	}

	bkt, err := resolveBucket(req.Bucket, span, limits)
	if err != nil {
		return nil, err
	}

	aggs, groupBy, err := resolveAggregations(req, def)
	if err != nil {
		return nil, err
	}

	plan := &QueryPlan{
		metric:  req.Metric,
		def:     def,
		start:   start,
		end:     end,
		bucket:  bkt,
		aggs:    aggs,
		groupBy: groupBy,
	}
	plan.cacheKey = plan.buildCacheKey(rangeKey)
	return plan, nil
}

func resolveWindow(req QueryRequest, limits QueryLimits, now time.Time) (time.Time, time.Time, string, error) {
	hasAbs := req.Start != nil || req.End != nil
	switch {
	case req.Range != "" && hasAbs:
		return time.Time{}, time.Time{}, "", verr("specify either range or start/end, not both")
	case req.Range != "":
		s, e, ok := resolveRange(req.Range, now, limits.MaxRange)
		if !ok {
			return time.Time{}, time.Time{}, "", verr("unknown range %q", req.Range)
		}
		return s, e, "range:" + req.Range, nil
	case hasAbs:
		if req.Start == nil {
			return time.Time{}, time.Time{}, "", verr("start is required when using absolute times")
		}
		s, err := time.Parse(time.RFC3339, *req.Start)
		if err != nil {
			return time.Time{}, time.Time{}, "", verr("invalid start: %v", err)
		}
		e := now
		if req.End != nil {
			e, err = time.Parse(time.RFC3339, *req.End)
			if err != nil {
				return time.Time{}, time.Time{}, "", verr("invalid end: %v", err)
			}
		}
		key := "abs:" + s.UTC().Format(time.RFC3339) + ":" + e.UTC().Format(time.RFC3339)
		return s, e, key, nil
	default:
		return now.Add(-24 * time.Hour), now, "range:24h", nil
	}
}

func resolveBucket(override string, span time.Duration, limits QueryLimits) (bucketDef, error) {
	if override == "" {
		return pickBucket(span, limits.TargetPoints), nil
	}
	b, ok := lookupBucket(override)
	if !ok {
		return bucketDef{}, verr("unknown bucket %q (allowed: %s)", override, bucketNames())
	}
	if int(span/b.Duration) > limits.MaxPoints {
		return bucketDef{}, verr("range too large for bucket %q (would exceed %d points); use a larger bucket or shorter range", override, limits.MaxPoints)
	}
	return b, nil
}

func resolveAggregations(req QueryRequest, def metricDef) ([]string, bool, error) {
	switch def.Type {
	case MetricTypeCount:
		if req.GroupBy == "" {
			return nil, false, nil
		}
		if req.GroupBy != "common_name" {
			return nil, false, verr("unknown group_by %q (allowed: common_name)", req.GroupBy)
		}
		return nil, true, nil
	default: // gauge
		if req.GroupBy != "" {
			return nil, false, verr("group_by is only valid for count metrics")
		}
		if len(req.Aggregations) == 0 {
			return defaultGaugeAggs, false, nil
		}
		var aggs []string
		seen := map[string]bool{}
		for _, a := range req.Aggregations {
			a = strings.ToLower(strings.TrimSpace(a))
			if !allowedGaugeAggs[a] {
				return nil, false, verr("unknown aggregation %q (allowed: avg, min, max, count, first, last)", a)
			}
			if !seen[a] {
				seen[a] = true
				aggs = append(aggs, a)
			}
		}
		return aggs, false, nil
	}
}

func (p *QueryPlan) buildCacheKey(rangeKey string) string {
	parts := []string{"query", p.metric, rangeKey, "b=" + p.bucket.Name}
	if len(p.aggs) > 0 {
		a := append([]string(nil), p.aggs...)
		sort.Strings(a)
		parts = append(parts, "agg="+strings.Join(a, ","))
	}
	if p.groupBy {
		parts = append(parts, "group=common_name")
	}
	return strings.Join(parts, "|")
}

//go:embed queries/getquery_gauge.pgsql.gotmpl
var getQueryGaugeTemplate string

//go:embed queries/getquery_count.pgsql.gotmpl
var getQueryCountTemplate string

var (
	getQueryGaugeTmpl = template.Must(template.New("getQueryGauge").Parse(getQueryGaugeTemplate))
	getQueryCountTmpl = template.Must(template.New("getQueryCount").Parse(getQueryCountTemplate))
)

type queryTemplateParams struct {
	Table          string
	Column         string
	SerialFiltered bool
	Serial         string
	GroupBy        bool
}

// RunQuery executes a planned query (cache first), returning the resolved series.
func (c *TimescaleClient) RunQuery(ctx context.Context, plan *QueryPlan) (*QueryResponse, error) {
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, c.queryCacheKey(plan)).Result()
		if err == nil {
			var cached QueryResponse
			if err := json.Unmarshal([]byte(res), &cached); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &cached, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	params := queryTemplateParams{
		Table:          plan.def.Table,
		Column:         plan.def.Column,
		SerialFiltered: plan.def.SerialFiltered,
		Serial:         airgradientSerial,
		GroupBy:        plan.groupBy,
	}

	tmpl := getQueryGaugeTmpl
	if plan.def.Type == MetricTypeCount {
		tmpl = getQueryCountTmpl
	}

	query := bytes.NewBuffer(nil)
	if err := tmpl.Execute(query, params); err != nil {
		return nil, fmt.Errorf("failed to execute query template: %w", err)
	}

	slog.Debug("query", slog.String("query", query.String()))

	rows, err := c.Pool.Query(ctx, query.String(), plan.bucket.Interval, plan.start, plan.end)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", plan.metric, err)
	}
	defer rows.Close()

	var points []QueryPoint
	if plan.def.Type == MetricTypeCount {
		points, err = scanCountRows(rows, plan.groupBy)
	} else {
		points, err = scanGaugeRows(rows, plan.aggs)
	}
	if err != nil {
		return nil, err
	}

	resp := &QueryResponse{
		Metric: plan.metric,
		Type:   plan.def.Type,
		Start:  plan.start,
		End:    plan.end,
		Bucket: plan.bucket.Name,
		Points: points,
	}
	if plan.groupBy {
		resp.GroupBy = "common_name"
	}

	if c.Dfly != nil {
		respJSON, err := json.Marshal(resp)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else if err := c.Dfly.GetClient().Set(ctx, c.queryCacheKey(plan), respJSON, c.Dfly.CacheResultsDuration).Err(); err != nil {
			slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
		}
	}

	return resp, nil
}

func (c *TimescaleClient) queryCacheKey(plan *QueryPlan) string {
	return fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, strconv.FormatUint(xxhash.Sum64String(plan.cacheKey), 16))
}

func scanGaugeRows(rows pgxRows, aggs []string) ([]QueryPoint, error) {
	want := map[string]bool{}
	for _, a := range aggs {
		want[a] = true
	}

	var points []QueryPoint
	for rows.Next() {
		var (
			t                        time.Time
			avg, min, max, fst, last *float64
			count                    int64
		)
		if err := rows.Scan(&t, &avg, &min, &max, &count, &fst, &last); err != nil {
			slog.Error("failed to scan row", slog.String("error", err.Error()))
			continue
		}
		p := QueryPoint{Time: t}
		if want["avg"] {
			p.Avg = avg
		}
		if want["min"] {
			p.Min = min
		}
		if want["max"] {
			p.Max = max
		}
		if want["first"] {
			p.First = fst
		}
		if want["last"] {
			p.Last = last
		}
		if want["count"] {
			c := count
			p.Count = &c
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func scanCountRows(rows pgxRows, groupBy bool) ([]QueryPoint, error) {
	var points []QueryPoint
	for rows.Next() {
		var (
			t     time.Time
			name  string
			count int64
		)
		var err error
		if groupBy {
			err = rows.Scan(&t, &name, &count)
		} else {
			err = rows.Scan(&t, &count)
		}
		if err != nil {
			slog.Error("failed to scan row", slog.String("error", err.Error()))
			continue
		}
		c := count
		p := QueryPoint{Time: t, Count: &c}
		if groupBy {
			p.CommonName = name
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// pgxRows is the subset of pgx.Rows used by the scanners.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}
