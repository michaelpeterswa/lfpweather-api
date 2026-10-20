package timescale

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

var forecastNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// runRows builds 24 hourly rows for one run. Rows are emitted in reverse lead
// order so tests also prove the result is sorted. withQuantiles fills 21 levels
// whose value equals the index, so p10 == 2 and p90 == 18.
func runRows(source string, issued time.Time, withQuantiles bool) []forecastRow {
	model := "model-" + source
	var rows []forecastRow
	for lead := 24; lead >= 1; lead-- {
		r := forecastRow{
			IssuedAt:    issued,
			Source:      source,
			ValidAt:     issued.Add(time.Duration(lead) * time.Hour),
			LeadHours:   lead,
			Temperature: 50 + float64(lead),
			Model:       &model,
		}
		if withQuantiles {
			q := make([]float32, 21)
			for i := range q {
				q[i] = float32(i)
			}
			r.Quantiles = q
		}
		rows = append(rows, r)
	}
	return rows
}

func TestSelectForecastPrefersConfiguredSource(t *testing.T) {
	issued := forecastNow.Add(-30 * time.Minute)
	rows := append(runRows("timestral_nws", issued, true), runRows("timestral", issued, true)...)

	got, err := selectForecast(rows, "timestral", forecastNow, 3*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != "timestral" {
		t.Fatalf("source = %q, want timestral", got.Source)
	}
	if got.Model != "model-timestral" {
		t.Errorf("model = %q", got.Model)
	}
	if len(got.Forecast) != 24 {
		t.Fatalf("len(forecast) = %d, want 24", len(got.Forecast))
	}
	for i, p := range got.Forecast {
		if p.LeadHours != i+1 {
			t.Fatalf("point %d lead = %d, want %d", i, p.LeadHours, i+1)
		}
	}
	p := got.Forecast[0]
	if p.P10 == nil || p.P90 == nil || *p.P10 != 2 || *p.P90 != 18 {
		t.Errorf("p10/p90 = %v/%v, want 2/18", p.P10, p.P90)
	}
	if p.Median != 51 {
		t.Errorf("median = %v, want 51", p.Median)
	}
}

func TestSelectForecastFallsBackWhenPreferredStale(t *testing.T) {
	rows := append(
		runRows("timestral", forecastNow.Add(-4*time.Hour), true),
		runRows("timestral_nws", forecastNow.Add(-1*time.Hour), true)...,
	)
	got, err := selectForecast(rows, "timestral", forecastNow, 3*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != "timestral_nws" {
		t.Fatalf("source = %q, want timestral_nws", got.Source)
	}
}

func TestSelectForecastNoneFresh(t *testing.T) {
	old := forecastNow.Add(-4 * time.Hour)
	rows := append(runRows("timestral", old, true), runRows("timestral_nws", old, true)...)
	rows = append(rows, runRows("nws", old, false)...)
	if _, err := selectForecast(rows, "timestral", forecastNow, 3*time.Hour); !errors.Is(err, ErrNoFreshForecast) {
		t.Fatalf("err = %v, want ErrNoFreshForecast", err)
	}
	if _, err := selectForecast(nil, "timestral", forecastNow, 3*time.Hour); !errors.Is(err, ErrNoFreshForecast) {
		t.Fatalf("empty: err = %v, want ErrNoFreshForecast", err)
	}
}

func TestSelectForecastAttachesSameRunNWS(t *testing.T) {
	issued := forecastNow.Add(-time.Hour)
	rows := runRows("timestral", issued, true)
	rows = append(rows, runRows("nws", issued, false)...)
	rows = append(rows, runRows("nws", issued.Add(-time.Hour), false)...)

	got, err := selectForecast(rows, "timestral", forecastNow, 3*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NWS) != 24 {
		t.Fatalf("len(nws) = %d, want 24", len(got.NWS))
	}
	for i, p := range got.NWS {
		if want := issued.Add(time.Duration(i+1) * time.Hour); !p.Time.Equal(want) {
			t.Fatalf("nws[%d].Time = %v, want %v", i, p.Time, want)
		}
	}
	if got.Forecast[0].P10 != nil && got.Forecast[0].Median == 0 {
		t.Error("forecast polluted by nws rows")
	}
	if len(got.Forecast) != 24 {
		t.Errorf("len(forecast) = %d, want 24", len(got.Forecast))
	}
}

func TestSelectForecastNoNWS(t *testing.T) {
	rows := runRows("timestral", forecastNow.Add(-time.Hour), true)
	got, err := selectForecast(rows, "timestral", forecastNow, 3*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NWS == nil || len(got.NWS) != 0 {
		t.Fatalf("NWS = %#v, want empty non-nil", got.NWS)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if string(m["nws"]) != "[]" {
		t.Errorf("nws json = %s, want []", m["nws"])
	}
	if got.Observed == nil {
		t.Error("Observed should be non-nil empty slice")
	}
}

func TestObservedWindow(t *testing.T) {
	issued := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	from, to := observedWindow(issued)
	if !from.Equal(issued.Add(-12*time.Hour)) || !to.Equal(issued) {
		t.Errorf("window = %v..%v, want issued-12h..issued", from, to)
	}
}
