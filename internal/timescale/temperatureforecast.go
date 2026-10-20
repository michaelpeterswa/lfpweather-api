package timescale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed queries/gettemperatureforecastruns.pgsql
var getTemperatureForecastRunsQuery string

//go:embed queries/gettemperatureforecastobserved.pgsql
var getTemperatureForecastObservedQuery string

const temperatureForecastCacheKey = "temperatureforecast"

// Quantile indexes into the 21-level array (0.01, 0.05, 0.10, ..., 0.95, 0.99).
const (
	quantileIdxP10 = 2
	quantileIdxP90 = 18
	quantileCount  = 21
)

const sourceNWS = "nws"

// ErrNoFreshForecast means no forecast run is recent enough to serve.
var ErrNoFreshForecast = errors.New("no fresh temperature forecast")

// ForecastPoint is one forecast hour. P10 and P90 are absent for sources
// without a predictive distribution.
type ForecastPoint struct {
	ValidAt   time.Time `json:"valid_at"`
	LeadHours int       `json:"lead_hours"`
	Median    float64   `json:"median"`
	P10       *float64  `json:"p10,omitempty"`
	P90       *float64  `json:"p90,omitempty"`
}

// TemperaturePoint is one hourly temperature value.
type TemperaturePoint struct {
	Time        time.Time `json:"time"`
	Temperature float64   `json:"temperature"`
}

// TemperatureForecast is the payload of the temperature forecast endpoint.
type TemperatureForecast struct {
	IssuedAt time.Time          `json:"issued_at"`
	Source   string             `json:"source"`
	Model    string             `json:"model"`
	Forecast []ForecastPoint    `json:"forecast"`
	NWS      []TemperaturePoint `json:"nws"`
	Observed []TemperaturePoint `json:"observed"`
}

// forecastRow is the scan target for the runs query.
type forecastRow struct {
	IssuedAt    time.Time
	Source      string
	ValidAt     time.Time
	LeadHours   int
	Temperature float64
	Quantiles   []float32
	Model       *string
}

// selectForecast picks the run to serve from rows holding the newest run per
// source. The preferred source wins if its run is no older than maxAge;
// otherwise the freshest fresh non-nws run wins. The nws rows with the chosen
// run's issued_at are attached as the reference series.
func selectForecast(rows []forecastRow, preferred string, now time.Time, maxAge time.Duration) (TemperatureForecast, error) {
	cutoff := now.Add(-maxAge)

	// newest issued_at per source
	newest := map[string]time.Time{}
	for _, r := range rows {
		if t, ok := newest[r.Source]; !ok || r.IssuedAt.After(t) {
			newest[r.Source] = r.IssuedAt
		}
	}

	chosen := ""
	var chosenAt time.Time
	if t, ok := newest[preferred]; ok && preferred != sourceNWS && !t.Before(cutoff) {
		chosen, chosenAt = preferred, t
	} else {
		for src, t := range newest {
			if src == sourceNWS || t.Before(cutoff) {
				continue
			}
			// newest wins; source name breaks ties deterministically
			if chosen == "" || t.After(chosenAt) || (t.Equal(chosenAt) && src < chosen) {
				chosen, chosenAt = src, t
			}
		}
	}
	if chosen == "" {
		return TemperatureForecast{}, ErrNoFreshForecast
	}

	out := TemperatureForecast{
		IssuedAt: chosenAt,
		Source:   chosen,
		Forecast: []ForecastPoint{},
		NWS:      []TemperaturePoint{},
		Observed: []TemperaturePoint{},
	}
	for _, r := range rows {
		if !r.IssuedAt.Equal(chosenAt) {
			continue
		}
		switch r.Source {
		case chosen:
			p := ForecastPoint{ValidAt: r.ValidAt, LeadHours: r.LeadHours, Median: r.Temperature}
			if len(r.Quantiles) == quantileCount {
				lo, hi := float64(r.Quantiles[quantileIdxP10]), float64(r.Quantiles[quantileIdxP90])
				p.P10, p.P90 = &lo, &hi
			}
			if out.Model == "" && r.Model != nil {
				out.Model = *r.Model
			}
			out.Forecast = append(out.Forecast, p)
		case sourceNWS:
			out.NWS = append(out.NWS, TemperaturePoint{Time: r.ValidAt, Temperature: r.Temperature})
		}
	}
	sort.Slice(out.Forecast, func(i, j int) bool { return out.Forecast[i].LeadHours < out.Forecast[j].LeadHours })
	sort.Slice(out.NWS, func(i, j int) bool { return out.NWS[i].Time.Before(out.NWS[j].Time) })
	return out, nil
}

// observedWindow returns the observation bounds for a run: the 12 hours up to
// its (hour-aligned) issued_at.
func observedWindow(issuedAt time.Time) (from, to time.Time) {
	return issuedAt.Add(-12 * time.Hour), issuedAt
}

// GetTemperatureForecast returns the freshest usable 24h temperature forecast
// with the NWS reference series and the last 12h of observations, using the
// dragonfly cache when it is configured.
func (c *TimescaleClient) GetTemperatureForecast(ctx context.Context, preferred string, maxAge time.Duration) (*TemperatureForecast, error) {
	var cacheKey string
	if c.Dfly != nil {
		cacheKey = fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, temperatureForecastCacheKey)
		res, err := c.Dfly.GetClient().Get(ctx, cacheKey).Result()
		if err == nil {
			var cached TemperatureForecast
			if err := json.Unmarshal([]byte(res), &cached); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &cached, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	rows, err := c.Pool.Query(ctx, getTemperatureForecastRunsQuery, maxAge.Seconds())
	if err != nil {
		return nil, fmt.Errorf("failed to query temperature forecast: %w", err)
	}
	defer rows.Close()

	var runs []forecastRow
	for rows.Next() {
		var r forecastRow
		var lead int16
		if err := rows.Scan(&r.IssuedAt, &r.Source, &r.ValidAt, &lead, &r.Temperature, &r.Quantiles, &r.Model); err != nil {
			slog.Error("failed to scan temperature forecast row", slog.String("error", err.Error()))
			continue
		}
		r.LeadHours = int(lead)
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate temperature forecast rows: %w", err)
	}

	now := time.Now()
	f, err := selectForecast(runs, preferred, now, maxAge)
	if err != nil {
		return nil, err
	}

	from, to := observedWindow(f.IssuedAt)
	obsRows, err := c.Pool.Query(ctx, getTemperatureForecastObservedQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query observed temperature: %w", err)
	}
	defer obsRows.Close()
	for obsRows.Next() {
		var p TemperaturePoint
		if err := obsRows.Scan(&p.Time, &p.Temperature); err != nil {
			slog.Error("failed to scan observed temperature row", slog.String("error", err.Error()))
			continue
		}
		f.Observed = append(f.Observed, p)
	}
	if err := obsRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate observed temperature rows: %w", err)
	}

	if c.Dfly != nil {
		b, err := json.Marshal(f)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else if err := c.Dfly.GetClient().Set(ctx, cacheKey, b, c.Dfly.CacheResultsDuration).Err(); err != nil {
			slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
		}
	}
	return &f, nil
}
