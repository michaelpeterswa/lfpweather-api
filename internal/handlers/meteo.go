package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"alpineworks.io/rfc9457"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type MeteoHandler struct {
	timescaleClient *timescale.TimescaleClient
}

func NewMeteoHandler(timescaleClient *timescale.TimescaleClient) *MeteoHandler {
	return &MeteoHandler{timescaleClient: timescaleClient}
}

// GetGDD serves GET /api/v1/gdd: the year-to-date growing degree day series.
func (s *MeteoHandler) GetGDD(w http.ResponseWriter, r *http.Request) {
	summary, err := s.timescaleClient.GetGDD(r.Context())
	if err != nil {
		writeMeteoProblem(w, r, "failed to get growing degree days", err.Error())
		return
	}
	writeMeteoJSON(w, r, summary, "growing degree days")
}

// GetET0 serves GET /api/v1/et0: the year-to-date reference evapotranspiration.
func (s *MeteoHandler) GetET0(w http.ResponseWriter, r *http.Request) {
	summary, err := s.timescaleClient.GetET0(r.Context())
	if err != nil {
		writeMeteoProblem(w, r, "failed to get reference evapotranspiration", err.Error())
		return
	}
	writeMeteoJSON(w, r, summary, "reference evapotranspiration")
}

// GetWBGT serves GET /api/v1/wbgt: the current wet bulb globe temperature.
func (s *MeteoHandler) GetWBGT(w http.ResponseWriter, r *http.Request) {
	reading, err := s.timescaleClient.GetWBGT(r.Context())
	if err != nil {
		writeMeteoProblem(w, r, "failed to get wet bulb globe temperature", err.Error())
		return
	}
	writeMeteoJSON(w, r, reading, "wet bulb globe temperature")
}

// GetZambretti serves GET /api/v1/zambretti: the current Zambretti forecast.
func (s *MeteoHandler) GetZambretti(w http.ResponseWriter, r *http.Request) {
	forecast, err := s.timescaleClient.GetZambretti(r.Context())
	if err != nil {
		writeMeteoProblem(w, r, "failed to get zambretti forecast", err.Error())
		return
	}
	writeMeteoJSON(w, r, forecast, "zambretti forecast")
}

func writeMeteoJSON(w http.ResponseWriter, r *http.Request, v any, what string) {
	res, err := json.Marshal(v)
	if err != nil {
		writeMeteoProblem(w, r, "failed to marshal "+what, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		slog.Error("failed to write "+what, slog.String("error", err.Error()))
	}
}

func writeMeteoProblem(w http.ResponseWriter, r *http.Request, title, detail string) {
	statusCode := http.StatusInternalServerError

	problem := rfc9457.NewRFC9457(
		rfc9457.WithTitle(title),
		rfc9457.WithDetail(detail),
		rfc9457.WithInstance(r.URL.Path),
		rfc9457.WithStatus(statusCode),
	)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)

	problemJSON, err := problem.ToJSON()
	if err != nil {
		slog.Error("failed to marshal problem", slog.String("error", err.Error()))
		return
	}
	if _, err := w.Write([]byte(problemJSON)); err != nil {
		slog.Error("failed to write problem", slog.String("error", err.Error()))
	}
}
