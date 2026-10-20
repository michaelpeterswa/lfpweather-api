package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type fakeForecastGetter struct {
	res       *timescale.TemperatureForecast
	err       error
	preferred string
	maxAge    time.Duration
}

func (f *fakeForecastGetter) GetTemperatureForecast(_ context.Context, preferred string, maxAge time.Duration) (*timescale.TemperatureForecast, error) {
	f.preferred, f.maxAge = preferred, maxAge
	return f.res, f.err
}

func serve(g *fakeForecastGetter) *httptest.ResponseRecorder {
	h := NewTemperatureForecastHandler(g, "timestral", 3*time.Hour)
	rec := httptest.NewRecorder()
	h.GetLatest(rec, httptest.NewRequest(http.MethodGet, "/api/v1/temperature_forecast/latest", nil))
	return rec
}

func TestTemperatureForecastOK(t *testing.T) {
	want := &timescale.TemperatureForecast{
		IssuedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Source:   "timestral",
		Model:    "m1",
		Forecast: []timescale.ForecastPoint{{ValidAt: time.Date(2026, 10, 19, 13, 0, 0, 0, time.UTC), LeadHours: 1, Median: 12.5}},
	}
	g := &fakeForecastGetter{res: want}
	rec := serve(g)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var got timescale.TemperatureForecast
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "timestral" || got.Model != "m1" || !got.IssuedAt.Equal(want.IssuedAt) || len(got.Forecast) != 1 || got.Forecast[0].Median != 12.5 {
		t.Fatalf("unexpected body: %+v", got)
	}
	if g.preferred != "timestral" || g.maxAge != 3*time.Hour {
		t.Fatalf("config not passed: %q %v", g.preferred, g.maxAge)
	}
}

func TestTemperatureForecastNotFound(t *testing.T) {
	rec := serve(&fakeForecastGetter{err: timescale.ErrNoFreshForecast})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type = %q", ct)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["title"] != "no fresh forecast" {
		t.Fatalf("title = %v", p["title"])
	}
}

func TestTemperatureForecastError(t *testing.T) {
	rec := serve(&fakeForecastGetter{err: errors.New("boom")})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type = %q", ct)
	}
}
