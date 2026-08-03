package timescale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed queries/getfiredangersummary.pgsql
var getFireDangerSummaryQuery string

// fireDangerSummaryCacheKey is the fixed cache key for the summary. The query
// takes no parameters, so a single key covers it.
const fireDangerSummaryCacheKey = "firedangersummary"

// FireDangerRating is the reader-facing verdict: an adjective class derived from
// where the current Energy Release Component sits in the station's own fire
// season distribution, plus a short plain-language explanation.
type FireDangerRating struct {
	// Class is one of Low, Moderate, High, Very High, Extreme.
	Class string `json:"class"`
	// Level is the class as an integer 0 (Low) to 4 (Extreme), for styling.
	Level int `json:"level"`
	// Headline is a short title, e.g. "Moderate fire danger".
	Headline string `json:"headline"`
	// Detail is one or two plain sentences that explain the class.
	Detail string `json:"detail"`
}

// ERCContext is the Energy Release Component with its season percentile context.
type ERCContext struct {
	Value      float64 `json:"value"`
	Percentile float64 `json:"percentile"` // 0..1, rank of Value in the season
	P50        float64 `json:"p50"`
	P80        float64 `json:"p80"`
	P90        float64 `json:"p90"`
	P97        float64 `json:"p97"`
	Max        float64 `json:"max"`
}

// BIContext is the Burning Index with its High/Extreme season breakpoints.
type BIContext struct {
	Value float64 `json:"value"`
	P90   float64 `json:"p90"`
	P97   float64 `json:"p97"`
}

// DroughtContext holds the slow drought and green-up signals.
type DroughtContext struct {
	KBDI float64 `json:"kbdi"` // 0..800
	GSI  float64 `json:"gsi"`  // 0..1
}

// FuelMoistureContext holds the dead and live fuel moisture percentages.
type FuelMoistureContext struct {
	Dead1hr    float64 `json:"dead_1hr"`
	Dead10hr   float64 `json:"dead_10hr"`
	Dead100hr  float64 `json:"dead_100hr"`
	Dead1000hr float64 `json:"dead_1000hr"`
	LiveHerb   float64 `json:"live_herbaceous"`
	LiveWoody  float64 `json:"live_woody"`
}

// ComponentsContext holds the two remaining NFDRS components.
type ComponentsContext struct {
	Spread   float64 `json:"spread"`
	Ignition float64 `json:"ignition"`
}

// FireDangerSummary is the payload of GET /api/v1/fire_danger/summary.
type FireDangerSummary struct {
	Time      time.Time `json:"time"`
	DeviceID  string    `json:"device_id"`
	FuelModel string    `json:"fuel_model"`

	Rating        FireDangerRating    `json:"rating"`
	EnergyRelease ERCContext          `json:"energy_release"`
	BurningIndex  BIContext           `json:"burning_index"`
	Drought       DroughtContext      `json:"drought"`
	FuelMoisture  FuelMoistureContext `json:"fuel_moisture"`
	Components    ComponentsContext   `json:"components"`
}

// classifyFireDanger maps the current ERC to an adjective class using the
// station's own season percentiles, then writes a short explanation that names
// the main driver (drought or fine dead fuel dryness).
//
// The class bands are: Low below the median, Moderate to the 80th percentile,
// High to the 90th, Very High to the 97th, and Extreme above it.
func classifyFireDanger(s FireDangerSummary) FireDangerRating {
	erc := s.EnergyRelease
	var class string
	var level int
	switch {
	case erc.Value >= erc.P97:
		class, level = "Extreme", 4
	case erc.Value >= erc.P90:
		class, level = "Very High", 3
	case erc.Value >= erc.P80:
		class, level = "High", 2
	case erc.Value >= erc.P50:
		class, level = "Moderate", 1
	default:
		class, level = "Low", 0
	}

	var base string
	switch level {
	case 4:
		base = "Energy release is in the top few percent for the season."
	case 3:
		base = "Energy release is in the top tenth for the season."
	case 2:
		base = "Energy release is high for the season."
	case 1:
		base = "Energy release is mid-range for the season."
	default:
		base = "Energy release is low for the season."
	}

	detail := base + driverClause(level, s.Drought.KBDI, s.FuelMoisture.Dead10hr)

	return FireDangerRating{
		Class:    class,
		Level:    level,
		Headline: class + " fire danger",
		Detail:   detail,
	}
}

// driverClause appends the single most notable driver of the current class:
// critically dry fine fuels, severe or elevated drought, or (at the low end)
// the recent moisture that is keeping fuels damp.
func driverClause(level int, kbdi, dead10hr float64) string {
	switch {
	case dead10hr < 8:
		return " Fine dead fuels are critically dry."
	case kbdi >= 600:
		return " Soil drought is severe."
	case kbdi >= 400:
		return " Drought is elevated."
	case level <= 1 && dead10hr > 17:
		return " Recent moisture is keeping the fine fuels damp."
	default:
		return ""
	}
}

// GetFireDangerSummary returns the latest reading and its season-calibrated
// danger rating, using the dragonfly cache when it is configured.
func (c *TimescaleClient) GetFireDangerSummary(ctx context.Context) (*FireDangerSummary, error) {
	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, fireDangerSummaryCacheKey)
		res, err := c.Dfly.GetClient().Get(ctx, cacheKey).Result()
		if err == nil {
			var summary FireDangerSummary
			if err := json.Unmarshal([]byte(res), &summary); err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			} else {
				return &summary, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	var s FireDangerSummary
	err := c.Pool.QueryRow(ctx, getFireDangerSummaryQuery).Scan(
		&s.Time, &s.DeviceID, &s.FuelModel,
		&s.EnergyRelease.Value, &s.BurningIndex.Value, &s.Components.Spread, &s.Components.Ignition,
		&s.Drought.KBDI, &s.Drought.GSI,
		&s.FuelMoisture.Dead1hr, &s.FuelMoisture.Dead10hr,
		&s.FuelMoisture.Dead100hr, &s.FuelMoisture.Dead1000hr,
		&s.FuelMoisture.LiveHerb, &s.FuelMoisture.LiveWoody,
		&s.EnergyRelease.P50, &s.EnergyRelease.P80, &s.EnergyRelease.P90, &s.EnergyRelease.P97,
		&s.EnergyRelease.Max, &s.EnergyRelease.Percentile,
		&s.BurningIndex.P90, &s.BurningIndex.P97,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get fire danger summary: %w", err)
	}

	s.Rating = classifyFireDanger(s)

	if c.Dfly != nil {
		cacheKey := fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, fireDangerSummaryCacheKey)
		summaryJSON, err := json.Marshal(s)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			if err := c.Dfly.GetClient().Set(ctx, cacheKey, summaryJSON, c.Dfly.CacheResultsDuration).Err(); err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return &s, nil
}
