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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlanQuery(tt.req, testLimits, testNow)
			if (err != nil) != tt.wantErr {
				t.Errorf("PlanQuery() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanQueryDefaults(t *testing.T) {
	plan, err := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testNow)
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

func TestPlanQueryCacheKeyStable(t *testing.T) {
	// same relative range → identical cache key regardless of the wall clock,
	// so relative queries still hit the cache within TTL.
	p1, _ := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testNow)
	p2, _ := PlanQuery(QueryRequest{Metric: "temperature", Range: "24h"}, testLimits, testNow.Add(3*time.Minute))
	if p1.cacheKey != p2.cacheKey {
		t.Errorf("cache keys differ for same relative range: %q vs %q", p1.cacheKey, p2.cacheKey)
	}
}
