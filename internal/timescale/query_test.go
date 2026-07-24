package timescale

import (
	"testing"
	"time"
)

func strptr(s string) *string { return &s }

var testLimits = QueryLimits{
	MaxRange:     43800 * time.Hour, // ~5y
	TargetPoints: 750,
	MaxPoints:    5000,
}

// fixed reference time so range math is deterministic.
var testNow = time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

func num() ColumnInfo  { return ColumnInfo{DataType: "double precision", Numeric: true} }
func text() ColumnInfo { return ColumnInfo{DataType: "text", Numeric: false} }

// testCatalog mimics what IntrospectCatalog would return for the allowlisted
// tables, including a text column and the two power tables.
var testCatalog = Catalog{Tables: map[string]map[string]ColumnInfo{
	"vantagepro2plus": {
		"temperature":                 num(),
		"barometer_sea_level":         num(),
		"solar_radiation":             num(),
		"wind_speed_last":             num(),
		"wind_speed_high_last_10_min": num(),
		"rain_rate_last":              num(),
		"rain_last_24_hour":           num(),
		"uv_index":                    num(),
		"humidity":                    num(),
		"dew_point":                   num(),
	},
	"airgradient":            {"rco2": num(), "nox_index": num(), "tvoc_index": num()},
	"airgradient_aqi":        {"aqi": num()},
	"litime":                 {"total_voltage": num(), "soc": num(), "soh": text(), "cell_voltages": {DataType: "jsonb"}},
	"renogychargecontroller": {"battery_voltage": num(), "charging_power": num(), "name": {DataType: "character varying"}},
	"birdnet":                {"common_name": text()},
}}

func TestPickBucket(t *testing.T) {
	tests := []struct {
		name string
		span time.Duration
		want string
	}{
		{"12h", 12 * time.Hour, "5m"},
		{"24h", 24 * time.Hour, "5m"},
		{"7d", 7 * 24 * time.Hour, "15m"},
		{"30d", 30 * 24 * time.Hour, "1h"},
		{"90d", 90 * 24 * time.Hour, "3h"},
		{"1y", 365 * 24 * time.Hour, "12h"},
		{"5y", 43800 * time.Hour, "1w"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickBucket(tt.span, testLimits.TargetPoints)
			if got.Name != tt.want {
				t.Errorf("pickBucket(%s) = %s, want %s", tt.span, got.Name, tt.want)
			}
			if pts := int(tt.span / got.Duration); pts > testLimits.TargetPoints {
				t.Errorf("pickBucket(%s) yields %d points, over target %d", tt.span, pts, testLimits.TargetPoints)
			}
		})
	}
}

func TestPlanQueryValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     QueryRequest
		wantErr bool
	}{
		{"unknown metric", QueryRequest{Metric: "unicorns", Range: "24h"}, true},
		{"gauge default", QueryRequest{Metric: "temperature", Range: "24h"}, false},
		{"count metric", QueryRequest{Metric: "birdnet", Range: "7d"}, false},
		{"count grouped", QueryRequest{Metric: "birdnet", Range: "7d", GroupBy: "common_name"}, false},
		{"bad group_by value", QueryRequest{Metric: "birdnet", Range: "7d", GroupBy: "species"}, true},
		{"group_by on gauge", QueryRequest{Metric: "temperature", Range: "24h", GroupBy: "common_name"}, true},
		{"range and abs both", QueryRequest{Metric: "temperature", Range: "24h", Start: strptr("2026-07-01T00:00:00Z")}, true},
		{"unknown range", QueryRequest{Metric: "temperature", Range: "forever"}, true},
		{"abs without start", QueryRequest{Metric: "temperature", End: strptr("2026-07-01T00:00:00Z")}, true},
		{"abs valid", QueryRequest{Metric: "temperature", Start: strptr("2026-07-01T00:00:00Z"), End: strptr("2026-07-10T00:00:00Z")}, false},
		{"bad start format", QueryRequest{Metric: "temperature", Start: strptr("july first")}, true},
		{"start after end", QueryRequest{Metric: "temperature", Start: strptr("2026-07-10T00:00:00Z"), End: strptr("2026-07-01T00:00:00Z")}, true},
		{"unknown aggregation", QueryRequest{Metric: "temperature", Range: "24h", Aggregations: []string{"median"}}, true},
		{"valid aggregations", QueryRequest{Metric: "temperature", Range: "24h", Aggregations: []string{"avg", "last"}}, false},
		{"unknown bucket", QueryRequest{Metric: "temperature", Range: "24h", Bucket: "2m"}, true},
		{"bucket too fine for range", QueryRequest{Metric: "temperature", Range: "all", Bucket: "5m"}, true},
		{"valid bucket override", QueryRequest{Metric: "temperature", Range: "7d", Bucket: "1h"}, false},
		{"dotted ref valid", QueryRequest{Metric: "vantagepro2plus.dew_point", Range: "24h"}, false},
		{"dotted ref new table", QueryRequest{Metric: "renogychargecontroller.battery_voltage", Range: "7d"}, false},
		{"dotted unknown column", QueryRequest{Metric: "litime.voltage", Range: "24h"}, true},
		{"dotted non-numeric column", QueryRequest{Metric: "litime.soh", Range: "24h"}, true},
		{"dotted jsonb column", QueryRequest{Metric: "litime.cell_voltages", Range: "24h"}, true},
		{"dotted unknown table", QueryRequest{Metric: "furnace.temperature", Range: "24h"}, true},
		{"dotted into count table", QueryRequest{Metric: "birdnet.common_name", Range: "24h"}, true},
		{"empty metric", QueryRequest{Range: "24h"}, true},
		{"litime alias-free numeric", QueryRequest{Metric: "litime.soc", Range: "30d"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlanQuery(tt.req, testLimits, testCatalog, testNow)
			if (err != nil) != tt.wantErr {
				t.Errorf("PlanQuery() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanQueryDefaults(t *testing.T) {
	plan, err := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testCatalog, testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.aggs) != 3 {
		t.Errorf("expected 3 default aggregations, got %v", plan.aggs)
	}
	if plan.bucket.Name != "5m" {
		t.Errorf("expected auto bucket 5m for 24h, got %s", plan.bucket.Name)
	}
	if plan.def.Type != MetricTypeGauge {
		t.Errorf("expected gauge type, got %s", plan.def.Type)
	}
}

func TestResolveRangePrevYear(t *testing.T) {
	start, end, ok := resolveRange("prev_year", testNow, testLimits.MaxRange)
	if !ok {
		t.Fatal("prev_year should resolve")
	}
	if start.Year() != 2025 || start.Month() != 1 || start.Day() != 1 {
		t.Errorf("prev_year start = %v, want 2025-01-01", start)
	}
	if end.Year() != 2026 || end.Month() != 1 || end.Day() != 1 {
		t.Errorf("prev_year end = %v, want 2026-01-01", end)
	}
}

func TestCatalogDescribe(t *testing.T) {
	fields := testCatalog.Describe()

	if len(fields.Tables) != len(tableRegistry) {
		t.Errorf("Describe() returned %d tables, want %d", len(fields.Tables), len(tableRegistry))
	}
	if fields.Tables["birdnet"].Type != "count" {
		t.Errorf("birdnet type = %q, want count", fields.Tables["birdnet"].Type)
	}
	if got := fields.Aliases["pressure"]; got != "vantagepro2plus.barometer_sea_level" {
		t.Errorf("pressure alias = %q, want vantagepro2plus.barometer_sea_level", got)
	}

	// litime.soh is text: present in the catalog but flagged non-numeric.
	var sawSOH bool
	for _, f := range fields.Tables["litime"].Columns {
		if f.Name == "soh" {
			sawSOH = true
			if f.Numeric {
				t.Error("soh should be reported as non-numeric")
			}
		}
	}
	if !sawSOH {
		t.Error("litime.soh missing from Describe() output")
	}
}

func TestPlanQueryCacheKeyStable(t *testing.T) {
	// same relative range → identical cache key regardless of the wall clock,
	// so relative queries still hit the cache within TTL.
	p1, _ := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testCatalog, testNow)
	p2, _ := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testCatalog, testNow.Add(3*time.Minute))
	if p1.cacheKey != p2.cacheKey {
		t.Errorf("cache keys differ for same relative range: %q vs %q", p1.cacheKey, p2.cacheKey)
	}
}
