package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"alpineworks.io/rfc9457"
	"github.com/michaelpeterswa/lfpweather-api/pkg/electricitymaps"
)

type ElectricityMapsHandler struct {
	ElectricityMapsClient *electricitymaps.ElectricityMapsClient
}

func NewElectricityMapsHandler(client *electricitymaps.ElectricityMapsClient) *ElectricityMapsHandler {
	return &ElectricityMapsHandler{
		ElectricityMapsClient: client,
	}
}

type ImportExportAdorned struct {
	ZoneName string `json:"zoneName"`
	Value    *int   `json:"value"`
}

type GetPowerBreakdownLatestResponse struct {
	Zone                      string    `json:"zone"`
	ZoneName                  string    `json:"zoneName"`
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
	PowerImportBreakdown  map[string]ImportExportAdorned `json:"powerImportBreakdown"`
	PowerExportBreakdown  map[string]ImportExportAdorned `json:"powerExportBreakdown"`
	FossilFreePercentage  *int                           `json:"fossilFreePercentage"`
	RenewablePercentage   *int                           `json:"renewablePercentage"`
	PowerConsumptionTotal *int                           `json:"powerConsumptionTotal"`
	PowerProductionTotal  *int                           `json:"powerProductionTotal"`
	PowerImportTotal      *int                           `json:"powerImportTotal"`
	PowerExportTotal      *int                           `json:"powerExportTotal"`
	IsEstimated           *bool                          `json:"isEstimated"`
	EstimationMethod      *string                        `json:"estimationMethod"`
}

func (h *ElectricityMapsHandler) GetPowerBreakdownLatest(w http.ResponseWriter, r *http.Request) {
	zones, err := h.ElectricityMapsClient.GetZones(r.Context(), false)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get zones"),
			rfc9457.WithDetail(fmt.Sprintf("error getting zones: %s", err.Error())),
			rfc9457.WithInstance(r.URL.Path),
			rfc9457.WithStatus(statusCode),
		)

		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(statusCode)

		problemJSON, err := problem.ToJSON()
		if err != nil {
			slog.Error("failed to marshal problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = w.Write([]byte(problemJSON))
		if err != nil {
			slog.Error("failed to write problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	breakdown, err := h.ElectricityMapsClient.GetPowerBreakdownLatest(r.Context(), electricitymaps.DefaultZone)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get power breakdown"),
			rfc9457.WithDetail(fmt.Sprintf("error getting power breakdown for zone %s: %s", electricitymaps.DefaultZone, err.Error())),
			rfc9457.WithInstance(r.URL.Path),
			rfc9457.WithStatus(statusCode),
		)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(statusCode)

		problemJSON, err := problem.ToJSON()
		if err != nil {
			slog.Error("failed to marshal problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = w.Write([]byte(problemJSON))
		if err != nil {
			slog.Error("failed to write problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	gpblr, err := translateGetPowerBreakdownLatestResponse(breakdown, zones)
	if err != nil {
		slog.Error("failed to translate GetPowerBreakdownLatestResponse", slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to translate response"),
			rfc9457.WithDetail(fmt.Sprintf("error translating GetPowerBreakdownLatestResponse: %s", err.Error())),
			rfc9457.WithInstance(r.URL.Path),
			rfc9457.WithStatus(http.StatusInternalServerError),
		)
		problemJSON, err := problem.ToJSON()
		if err != nil {
			slog.Error("failed to marshal problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = w.Write([]byte(problemJSON))
		if err != nil {
			slog.Error("failed to write problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	gpblrJson, err := json.Marshal(gpblr)
	if err != nil {
		slog.Error("failed to marshal GetPowerBreakdownLatestResponse", slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to marshal response"),
			rfc9457.WithDetail(fmt.Sprintf("error marshaling GetPowerBreakdownLatestResponse: %s", err.Error())),
			rfc9457.WithInstance(r.URL.Path),
			rfc9457.WithStatus(http.StatusInternalServerError),
		)

		problemJSON, err := problem.ToJSON()
		if err != nil {
			slog.Error("failed to marshal problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = w.Write([]byte(problemJSON))
		if err != nil {
			slog.Error("failed to write problem", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(gpblrJson)
	if err != nil {
		slog.Error("failed to write response", slog.String("error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func translateGetPowerBreakdownLatestResponse(resp *electricitymaps.GetPowerBreakdownLatestResponse, zones electricitymaps.GetZonesResponse) (*GetPowerBreakdownLatestResponse, error) {
	if resp == nil {
		slog.Error("received nil GetPowerBreakdownLatestResponse")
		return nil, fmt.Errorf("received nil GetPowerBreakdownLatestResponse")
	}

	var gpblr GetPowerBreakdownLatestResponse

	gpblr.PowerImportBreakdown = make(map[string]ImportExportAdorned)
	gpblr.PowerExportBreakdown = make(map[string]ImportExportAdorned)

	for key, imports := range resp.PowerImportBreakdown {
		if importAdorned, ok := zones[key]; ok {
			gpblr.PowerImportBreakdown[key] = ImportExportAdorned{
				ZoneName: importAdorned.ZoneName,
				Value:    imports,
			}
		} else {
			slog.Warn("zone not found in import breakdown", slog.String("zone", key))
		}
	}
	for key, exports := range resp.PowerExportBreakdown {
		if exportAdorned, ok := zones[key]; ok {
			gpblr.PowerExportBreakdown[key] = ImportExportAdorned{
				ZoneName: exportAdorned.ZoneName,
				Value:    exports,
			}
		} else {
			slog.Warn("zone not found in export breakdown", slog.String("zone", key))
		}
	}

	if zoneName, ok := zones[resp.Zone]; ok {
		gpblr.ZoneName = zoneName.ZoneName
	} else {
		slog.Warn("zone not found", slog.String("zone", resp.Zone))
	}

	gpblr.Zone = resp.Zone
	gpblr.Datetime = resp.Datetime
	gpblr.UpdatedAt = resp.UpdatedAt
	gpblr.CreatedAt = resp.CreatedAt
	gpblr.PowerConsumptionBreakdown = resp.PowerConsumptionBreakdown
	gpblr.PowerProductionBreakdown = resp.PowerProductionBreakdown
	gpblr.FossilFreePercentage = resp.FossilFreePercentage
	gpblr.RenewablePercentage = resp.RenewablePercentage
	gpblr.PowerConsumptionTotal = resp.PowerConsumptionTotal
	gpblr.PowerProductionTotal = resp.PowerProductionTotal
	gpblr.PowerImportTotal = resp.PowerImportTotal
	gpblr.PowerExportTotal = resp.PowerExportTotal
	gpblr.IsEstimated = resp.IsEstimated
	gpblr.EstimationMethod = resp.EstimationMethod

	return &gpblr, nil
}
