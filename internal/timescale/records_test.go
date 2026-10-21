package timescale

import (
	"strings"
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

// TestBuildRecordScans checks the curated set groups into one scan per table,
// in recordDefs order, with a low only where one is meaningful.
func TestBuildRecordScans(t *testing.T) {
	records, scans := buildRecordScans(recordsTestCatalog)

	if len(records) != len(recordDefs) {
		t.Fatalf("got %d records, want %d", len(records), len(recordDefs))
	}
	if len(scans) != 2 || scans[0].Table != "vantagepro2plus" || scans[1].Table != "fire_danger" {
		t.Fatalf("scans = %+v, want vantagepro2plus then fire_danger", scans)
	}
	if scans[0].ExcludeSpinup || !scans[1].ExcludeSpinup {
		t.Error("only fire_danger should exclude spin-up rows")
	}

	// 8 highs + 3 lows (temperature, humidity, pressure) on vantagepro2plus.
	if got := len(scans[0].Extremes); got != 11 {
		t.Errorf("vantagepro2plus extremes = %d, want 11", got)
	}
	lows := map[string]bool{}
	for _, s := range scans {
		for _, ex := range s.Extremes {
			if ex.high != (ex.Agg == "max") {
				t.Errorf("%s: high=%v but agg=%s", ex.Column, ex.high, ex.Agg)
			}
			if !ex.high {
				lows[records[ex.record].Metric] = true
			}
		}
	}
	for _, rd := range recordDefs {
		if lows[rd.Metric] != rd.Low {
			t.Errorf("%s: has low = %v, want %v", rd.Metric, lows[rd.Metric], rd.Low)
		}
	}
}

func TestRecordTemplates(t *testing.T) {
	scan := &recordScan{
		Table:         "fire_danger",
		ExcludeSpinup: true,
		Extremes: []recordExtreme{
			{Column: "burning_index", Agg: "max", Param: 3},
			{Column: "burning_index", Agg: "min", Param: 4},
		},
	}

	var values, times strings.Builder
	if err := getRecordValuesTmpl.Execute(&values, scan); err != nil {
		t.Fatal(err)
	}
	if err := getRecordTimesTmpl.Execute(&times, scan); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"(max(burning_index))::float8",
		",(min(burning_index))::float8",
		"FROM\n    sensors.fire_danger",
		"AND NOT spinup",
	} {
		if !strings.Contains(values.String(), want) {
			t.Errorf("values query missing %q:\n%s", want, values.String())
		}
	}
	for _, want := range []string{
		`min("time") FILTER (WHERE burning_index::float8 = $3)`,
		`,min("time") FILTER (WHERE burning_index::float8 = $4)`,
	} {
		if !strings.Contains(times.String(), want) {
			t.Errorf("times query missing %q:\n%s", want, times.String())
		}
	}
	if strings.Contains(values.String(), "serial_number") {
		t.Error("unfiltered table should not filter by serial")
	}
}

func TestMergeRecords(t *testing.T) {
	ext := func(v float64, y int) *RecordExtreme {
		return &RecordExtreme{Value: v, Time: laDate(y, 7, 1, 12)}
	}
	periods := []*RecordsResponse{
		{Records: []MetricRecord{
			{Metric: "temperature", High: ext(95, 2024), Low: ext(20, 2024)},
			{Metric: "erc"}, // no reading this year
		}},
		{Records: []MetricRecord{
			{Metric: "temperature", High: ext(101, 2025), Low: ext(20, 2025)},
			{Metric: "erc", High: ext(60, 2025)},
		}},
		{Records: []MetricRecord{
			// listed out of curated order on purpose
			{Metric: "erc", High: ext(55, 2026)},
			{Metric: "temperature", High: ext(101, 2026), Low: ext(25, 2026)},
		}},
	}

	got := mergeRecords(periods)

	if len(got) != 2 || got[0].Metric != "temperature" || got[1].Metric != "erc" {
		t.Fatalf("merged = %+v, want temperature then erc", got)
	}
	temp := got[0]
	if temp.High.Value != 101 || temp.High.Time.Year() != 2025 {
		t.Errorf("high = %+v, want 101 from 2025 (a tie keeps the earlier)", temp.High)
	}
	if temp.Low.Value != 20 || temp.Low.Time.Year() != 2024 {
		t.Errorf("low = %+v, want 20 from 2024 (a tie keeps the earlier)", temp.Low)
	}
	if got[1].High.Value != 60 || got[1].Low != nil {
		t.Errorf("erc = %+v, want high 60 and no low", got[1])
	}
}

// TestRecordPlausibilityBound checks a bounded high filters out readings above
// its bound, and that the bound reaches the scan from recordDefs.
func TestRecordPlausibilityBound(t *testing.T) {
	_, scans := buildRecordScans(recordsTestCatalog)
	var bounded *recordExtreme
	for _, ex := range scans[0].Extremes {
		if ex.Column == "wind_speed_high_last_10_min" {
			ex := ex
			bounded = &ex
		}
	}
	if bounded == nil || bounded.Max != 100 || len(bounded.Exclude) != 1 {
		t.Fatalf("wind gust extreme = %+v, want Max 100 and one excluded window", bounded)
	}

	var values strings.Builder
	scan := &recordScan{Table: "vantagepro2plus", Extremes: []recordExtreme{*bounded}}
	if err := getRecordValuesTmpl.Execute(&values, scan); err != nil {
		t.Fatal(err)
	}
	filter := `wind_speed_high_last_10_min <= 100 AND NOT ("time" >= '2026-04-15T17:40:00Z' AND "time" < '2026-04-15T17:50:00Z')`
	want := "(max(wind_speed_high_last_10_min) FILTER (WHERE " + filter + "))::float8"
	if !strings.Contains(values.String(), want) {
		t.Errorf("values query missing %q:\n%s", want, values.String())
	}

	// The times query must apply the same filter, or a value seen both inside
	// and outside an excluded window could be timed to the glitch.
	var times strings.Builder
	bounded.Param = 3
	scan.Extremes = []recordExtreme{*bounded}
	if err := getRecordTimesTmpl.Execute(&times, scan); err != nil {
		t.Fatal(err)
	}
	if want := "= $3 AND " + filter + ")"; !strings.Contains(times.String(), want) {
		t.Errorf("times query missing %q:\n%s", want, times.String())
	}
}
