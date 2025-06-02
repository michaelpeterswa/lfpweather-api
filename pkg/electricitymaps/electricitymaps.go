package electricitymaps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/michaelpeterswa/lfpweather-api/internal/dragonfly"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultBaseUrl = "https://api.electricitymap.org/v3"
	DefaultZone    = "US-NW-SCL" // Seattle City Light (only zone allowed with my key)
)

type ElectricityMapsClient struct {
	apiKey  string
	baseUrl string
	client  *http.Client
	dfly    *dragonfly.DragonflyClient
}

type ElectricityMapsClientOption func(*ElectricityMapsClient)

func NewElectricityMapsClient(apiKey string, opts ...ElectricityMapsClientOption) *ElectricityMapsClient {
	client := &ElectricityMapsClient{
		apiKey:  apiKey,
		baseUrl: DefaultBaseUrl,
		client:  http.DefaultClient,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func WithBaseUrl(baseUrl string) ElectricityMapsClientOption {
	return func(c *ElectricityMapsClient) {
		c.baseUrl = baseUrl
	}
}

func WithHttpClient(client *http.Client) ElectricityMapsClientOption {
	return func(c *ElectricityMapsClient) {
		c.client = client
	}
}

func WithDragonflyClient(dfly *dragonfly.DragonflyClient) ElectricityMapsClientOption {
	return func(c *ElectricityMapsClient) {
		c.dfly = dfly
	}
}

type Zone struct {
	CountryName string `json:"countryName"`
	ZoneName    string `json:"zoneName"`
	DisplayName string `json:"displayName"`
	Access      string `json:"access"`
}

type GetZonesResponse map[string]Zone

func (emc *ElectricityMapsClient) GetZones(ctx context.Context, useApiKey bool) (GetZonesResponse, error) {
	if emc.dfly != nil {
		// Try to get zones from Dragonfly cache
		res, err := emc.dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s", emc.dfly.KeyPrefix, "zones")).Result()
		if err == nil {
			var getZonesResponse GetZonesResponse
			err := json.Unmarshal([]byte(res), &getZonesResponse)
			if err != nil {
				slog.Error("failed to unmarshal zones from dragonfly", slog.String("error", err.Error()))
			}
			return getZonesResponse, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get zones from dragonfly", slog.String("error", err.Error()))
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/zones", emc.baseUrl), nil)
	if err != nil {
		return nil, err
	}

	// not setting api key allows all zones to be fetched
	if useApiKey {
		req.Header.Set("auth-token", emc.apiKey)
	}

	resp, err := emc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get zones: %s", resp.Status)
	}

	var zones GetZonesResponse
	if err := json.NewDecoder(resp.Body).Decode(&zones); err != nil {
		return nil, err
	}

	if emc.dfly != nil {
		getZonesResponseJSON, err := json.Marshal(zones)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			err := emc.dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s", emc.dfly.KeyPrefix, "zones"), getZonesResponseJSON, emc.dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return zones, nil
}

type GetPowerBreakdownLatestResponse struct {
	Zone                      string    `json:"zone"`
	Datetime                  time.Time `json:"datetime"`
	UpdatedAt                 time.Time `json:"updatedAt"`
	CreatedAt                 time.Time `json:"createdAt"`
	PowerConsumptionBreakdown struct {
		Nuclear          *int `json:"nuclear"`
		Geothermal       *int `json:"geothermal"`
		Biomass          *int `json:"biomass"`
		Coal             *int `json:"coal"`
		Wind             *int `json:"wind"`
		Solar            *int `json:"solar"`
		Hydro            *int `json:"hydro"`
		Gas              *int `json:"gas"`
		Oil              *int `json:"oil"`
		Unknown          *int `json:"unknown"`
		HydroDischarge   *int `json:"hydro discharge"`
		BatteryDischarge *int `json:"battery discharge"`
	} `json:"powerConsumptionBreakdown"`
	PowerProductionBreakdown struct {
		Nuclear          *int `json:"nuclear"`
		Geothermal       *int `json:"geothermal"`
		Biomass          *int `json:"biomass"`
		Coal             *int `json:"coal"`
		Wind             *int `json:"wind"`
		Solar            *int `json:"solar"`
		Hydro            *int `json:"hydro"`
		Gas              *int `json:"gas"`
		Oil              *int `json:"oil"`
		Unknown          *int `json:"unknown"`
		HydroDischarge   *int `json:"hydro discharge"`
		BatteryDischarge *int `json:"battery discharge"`
	} `json:"powerProductionBreakdown"`
	PowerImportBreakdown  map[string]*int `json:"powerImportBreakdown"`
	PowerExportBreakdown  map[string]*int `json:"powerExportBreakdown"`
	FossilFreePercentage  *int            `json:"fossilFreePercentage"`
	RenewablePercentage   *int            `json:"renewablePercentage"`
	PowerConsumptionTotal *int            `json:"powerConsumptionTotal"`
	PowerProductionTotal  *int            `json:"powerProductionTotal"`
	PowerImportTotal      *int            `json:"powerImportTotal"`
	PowerExportTotal      *int            `json:"powerExportTotal"`
	IsEstimated           *bool           `json:"isEstimated"`
	EstimationMethod      *string         `json:"estimationMethod"`
}

func (emc *ElectricityMapsClient) GetPowerBreakdownLatest(ctx context.Context, zone string) (*GetPowerBreakdownLatestResponse, error) {
	if emc.dfly != nil {
		// Try to get power breakdown from Dragonfly cache
		res, err := emc.dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s-%s", emc.dfly.KeyPrefix, "power-breakdown-latest", zone)).Result()
		if err == nil {
			var getPowerBreakdownLatest GetPowerBreakdownLatestResponse
			err := json.Unmarshal([]byte(res), &getPowerBreakdownLatest)
			if err != nil {
				slog.Error("failed to unmarshal power breakdown from dragonfly", slog.String("error", err.Error()))
			}
			return &getPowerBreakdownLatest, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get power breakdown from dragonfly", slog.String("error", err.Error()))
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/power-breakdown/latest?zone=%s", emc.baseUrl, zone), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("auth-token", emc.apiKey)

	resp, err := emc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get power breakdown: %s", resp.Status)
	}

	var breakdown GetPowerBreakdownLatestResponse
	if err := json.NewDecoder(resp.Body).Decode(&breakdown); err != nil {
		return nil, err
	}

	if emc.dfly != nil {
		breakdownJSON, err := json.Marshal(breakdown)
		if err != nil {
			slog.Error("failed to marshal power breakdown to dragonfly", slog.String("error", err.Error()))
		} else {
			err := emc.dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s-%s", emc.dfly.KeyPrefix, "power-breakdown-latest", zone), breakdownJSON, emc.dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set power breakdown to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return &breakdown, nil
}
