package timescale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "embed"

	"github.com/michaelpeterswa/lfpweather-api/internal/meteo"
	"github.com/redis/go-redis/v9"
)

//go:embed queries/getgdd.pgsql
var getGDDQuery string

//go:embed queries/getzambretti.pgsql
var getZambrettiQuery string

// gddBaseF is the growing degree day base temperature in degrees Fahrenheit.
// 50 F is the general agronomic and pest-phenology default.
const gddBaseF = 50.0

// siteLSTOffset is the site's fixed local standard time offset (PST). It is used
// only to pick the calendar month for the Zambretti seasonal adjustment, so a
// fixed offset is enough and avoids a zoneinfo dependency.
const siteLSTOffset = -8 * time.Hour

const (
	gddCacheKey       = "gdd"
	zambrettiCacheKey = "zambretti"
)

// GDDPoint is one day of the growing degree day series.
type GDDPoint struct {
	Date        string  `json:"date"` // YYYY-MM-DD, local
	GDD         float64 `json:"gdd"`
	Accumulated float64 `json:"accumulated"`
}

// GDDSummary is the payload of GET /api/v1/gdd.
type GDDSummary struct {
	BaseF float64    `json:"base_f"`
	Since string     `json:"since"` // YYYY-MM-DD of the first day
	AsOf  string     `json:"as_of"` // YYYY-MM-DD of the last day
	Total float64    `json:"total"` // accumulated GDD for the year to date
	Daily []GDDPoint `json:"daily"`
}

// ZambrettiForecast is the payload of GET /api/v1/zambretti.
type ZambrettiForecast struct {
	Time            time.Time `json:"time"`
	Code            string    `json:"code"` // A..Z
	Text            string    `json:"text"`
	Trend           string    `json:"trend"` // rising, steady, falling
	PressureHPa     float64   `json:"pressure_hpa"`
	TrendHPaPerHour float64   `json:"trend_hpa_per_hour"`
	WindDirDeg      float64   `json:"wind_dir_deg"`
}

// GetGDD returns the year-to-date growing degree day series (base 50 F) using
// the dragonfly cache when configured.
func (c *TimescaleClient) GetGDD(ctx context.Context) (*GDDSummary, error) {
	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, gddCacheKey)
		if res, err := c.Dfly.GetClient().Get(ctx, cacheKey).Result(); err == nil {
			var summary GDDSummary
			if err := json.Unmarshal([]byte(res), &summary); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &summary, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	rows, err := c.Pool.Query(ctx, getGDDQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query growing degree days: %w", err)
	}
	defer rows.Close()

	summary := GDDSummary{BaseF: gddBaseF, Daily: []GDDPoint{}}
	var accumulated float64
	for rows.Next() {
		var day time.Time
		var tmin, tmax float64
		if err := rows.Scan(&day, &tmin, &tmax); err != nil {
			slog.Error("failed to scan gdd row", slog.String("error", err.Error()))
			continue
		}
		gdd := meteo.GrowingDegreeDaysF(tmax, tmin, gddBaseF)
		accumulated += gdd
		summary.Daily = append(summary.Daily, GDDPoint{
			Date:        day.Format("2006-01-02"),
			GDD:         gdd,
			Accumulated: accumulated,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate gdd rows: %w", err)
	}

	summary.Total = accumulated
	if len(summary.Daily) > 0 {
		summary.Since = summary.Daily[0].Date
		summary.AsOf = summary.Daily[len(summary.Daily)-1].Date
	}

	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, gddCacheKey)
		if b, err := json.Marshal(summary); err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else if err := c.Dfly.GetClient().Set(ctx, cacheKey, b, c.Dfly.CacheResultsDuration).Err(); err != nil {
			slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
		}
	}

	return &summary, nil
}

// GetZambretti returns the current Zambretti forecast, using the dragonfly cache
// when configured.
func (c *TimescaleClient) GetZambretti(ctx context.Context) (*ZambrettiForecast, error) {
	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, zambrettiCacheKey)
		if res, err := c.Dfly.GetClient().Get(ctx, cacheKey).Result(); err == nil {
			var forecast ZambrettiForecast
			if err := json.Unmarshal([]byte(res), &forecast); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &forecast, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	var t time.Time
	var pInHg, windDir float64
	var pastInHg *float64
	err := c.Pool.QueryRow(ctx, getZambrettiQuery).Scan(&t, &pInHg, &windDir, &pastInHg)
	if err != nil {
		return nil, fmt.Errorf("failed to query zambretti inputs: %w", err)
	}

	pressureHPa := pInHg * meteo.InHgToHPa

	// Trend in hPa per hour over the three-hour window. Absent history is steady.
	var trendHPaPerHour float64
	if pastInHg != nil {
		trendHPaPerHour = (pressureHPa - *pastInHg*meteo.InHgToHPa) / 3.0
	}

	month := t.Add(siteLSTOffset).Month()
	code, text := meteo.Zambretti(pressureHPa, windDir, month, trendHPaPerHour)

	forecast := ZambrettiForecast{
		Time:            t,
		Code:            code,
		Text:            text,
		Trend:           trendLabel(trendHPaPerHour),
		PressureHPa:     pressureHPa,
		TrendHPaPerHour: trendHPaPerHour,
		WindDirDeg:      windDir,
	}

	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, zambrettiCacheKey)
		if b, err := json.Marshal(forecast); err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else if err := c.Dfly.GetClient().Set(ctx, cacheKey, b, c.Dfly.CacheResultsDuration).Err(); err != nil {
			slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
		}
	}

	return &forecast, nil
}

// trendLabel names the pressure trend using the same 0.1 hPa/hour deadband as
// the Zambretti algorithm.
func trendLabel(hPaPerHour float64) string {
	switch {
	case hPaPerHour >= 0.1:
		return "rising"
	case hPaPerHour <= -0.1:
		return "falling"
	default:
		return "steady"
	}
}
