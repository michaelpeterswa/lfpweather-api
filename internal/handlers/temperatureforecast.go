package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type temperatureForecastGetter interface {
	GetTemperatureForecast(ctx context.Context, preferred string, maxAge time.Duration) (*timescale.TemperatureForecast, error)
}

type TemperatureForecastHandler struct {
	getter    temperatureForecastGetter
	preferred string
	maxAge    time.Duration
}

func NewTemperatureForecastHandler(getter temperatureForecastGetter, preferred string, maxAge time.Duration) *TemperatureForecastHandler {
	return &TemperatureForecastHandler{getter: getter, preferred: preferred, maxAge: maxAge}
}

// GetLatest serves GET /api/v1/temperature_forecast/latest.
func (h *TemperatureForecastHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	forecast, err := h.getter.GetTemperatureForecast(r.Context(), h.preferred, h.maxAge)
	if err != nil {
		if errors.Is(err, timescale.ErrNoFreshForecast) {
			writeProblem(w, r, http.StatusNotFound, "no fresh forecast", "no temperature forecast is recent enough to serve")
			return
		}
		slog.Error("failed to get temperature forecast", slog.String("error", err.Error()))
		writeProblem(w, r, http.StatusInternalServerError, "failed to get temperature forecast", "an internal error occurred")
		return
	}

	res, err := json.Marshal(forecast)
	if err != nil {
		slog.Error("failed to marshal temperature forecast", slog.String("error", err.Error()))
		writeProblem(w, r, http.StatusInternalServerError, "failed to marshal temperature forecast", "an internal error occurred")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		slog.Error("failed to write temperature forecast", slog.String("error", err.Error()))
	}
}
