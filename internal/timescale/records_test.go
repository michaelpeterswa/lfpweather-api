package timescale

import (
	"testing"
	"time"
)

// recordsTestCatalog covers every column the curated record set names,
// including the fire_danger table (absent from the query_test catalog).
var recordsTestCatalog = Catalog{Tables: map[string]map[string]ColumnInfo{
	"vantagepro2plus": {
		"temperature":                 num(),
		"humidity":                    num(),
		"barometer_sea_level":         num(),
		"wind_speed_high_last_10_min": num(),
		"rain_rate_last":              num(),
		"rain_last_24_hour":           num(),
		"solar_radiation":             num(),
		"uv_index":                    num(),
	},
	"fire_danger": {
		"energy_release_component": num(),
		"burning_index":            num(),
	},
}}

// laDate builds an instant at the station timezone, so the expected civil-day
// boundaries in the tests read the same way the endpoint computes them.
func laDate(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, recordsLocation)
}

func TestResolvePeriod(t *testing.T) {
	// A mid-August anchor: 2026-08-13 is a Thursday; the station is on PDT.
	at := laDate(2026, 8, 13, 10)
	// now is well past the anchor's periods, so day/week/month all complete.
	now := laDate(2026, 9, 1, 0).UTC()

	tests := []struct {
		name         string
		period       string
		wantStart    time.Time
		wantEnd      time.Time
		wantComplete bool
		wantErr      bool
	}{
		{"day", "day", laDate(2026, 8, 13, 0), laDate(2026, 8, 14, 0), true, false},
		{"week starts monday", "week", laDate(2026, 8, 10, 0), laDate(2026, 8, 17, 0), true, false},
		{"month", "month", laDate(2026, 8, 1, 0), laDate(2026, 9, 1, 0), true, false},
		{"year", "year", laDate(2026, 1, 1, 0), laDate(2027, 1, 1, 0), false, false},
		{"all never complete", "all", recordsAllStart, now, false, false},
		{"unknown period", "decade", time.Time{}, time.Time{}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, complete, err := resolvePeriod(tt.period, at, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolvePeriod() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !start.Equal(tt.wantStart) {
				t.Errorf("start = %v, want %v", start, tt.wantStart)
			}
			if !end.Equal(tt.wantEnd) {
				t.Errorf("end = %v, want %v", end, tt.wantEnd)
			}
			if complete != tt.wantComplete {
				t.Errorf("complete = %v, want %v", complete, tt.wantComplete)
			}
		})
	}
}

// TestResolvePeriodCompletion checks the completion flag flips exactly when a
// period's end has elapsed. The month containing the anchor is in progress; the
// prior month is complete.
func TestResolvePeriodCompletion(t *testing.T) {
	now := laDate(2026, 8, 13, 10).UTC()

	_, _, currentComplete, err := resolvePeriod("month", now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if currentComplete {
		t.Error("current month should not be complete")
	}

	priorAnchor := laDate(2026, 7, 15, 0)
	_, _, priorComplete, err := resolvePeriod("month", priorAnchor, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !priorComplete {
		t.Error("prior month should be complete")
	}
}

func TestParseAt(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"rfc3339", "2025-07-15T12:00:00Z", false},
		{"date only", "2025-07-15", false},
		{"garbage", "july fifteenth", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAt(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAt(%q) err = %v, wantErr = %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

// TestParseAtDateIsCivil confirms a plain date is read at the station timezone,
// not UTC. A UTC-midnight read would land on the previous civil day.
func TestParseAtDateIsCivil(t *testing.T) {
	at, err := ParseAt("2025-07-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	start, _, _, err := resolvePeriod("day", at, laDate(2026, 1, 1, 0).UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := laDate(2025, 7, 15, 0)
	if !start.Equal(want) {
		t.Errorf("day start = %v, want %v", start, want)
	}
}

// TestRecordDefsResolve guards against a typo in the curated record set: every
// metric must resolve against the column catalog, or the page silently drops a
// row.
func TestRecordDefsResolve(t *testing.T) {
	for _, rd := range recordDefs {
		if _, err := resolveMetric(rd.Metric, recordsTestCatalog); err != nil {
			t.Errorf("record metric %q does not resolve: %v", rd.Metric, err)
		}
	}
}
