package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"alpineworks.io/rfc9457"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type WeatherHandler struct {
	timescaleClient *timescale.TimescaleClient
}

func NewWeatherHandler(timescaleClient *timescale.TimescaleClient) *WeatherHandler {
	return &WeatherHandler{timescaleClient: timescaleClient}
}

func (s *WeatherHandler) Close() {
	s.timescaleClient.Close()
}

func (s *WeatherHandler) GetGeneric24h(w http.ResponseWriter, r *http.Request, columnName string) {
	temperatures, err := s.timescaleClient.GetColumn24h(r.Context(), columnName)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error getting data for column %s: %s", columnName, err.Error())),
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

		w.Write([]byte(problemJSON))
		return
	}

	res, err := json.Marshal(temperatures)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to marshal 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error marshalling data for column %s: %s", columnName, err.Error())),
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

		w.Write([]byte(problemJSON))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(res)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to write 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error writing data for column %s: %s", columnName, err.Error())),
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

		w.Write([]byte(problemJSON))
		return
	}
}

func (s *WeatherHandler) GetTemperature24h(w http.ResponseWriter, r *http.Request) {
	s.GetGeneric24h(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidity24h(w http.ResponseWriter, r *http.Request) {
	s.GetGeneric24h(w, r, "humidity")
}

func (s *WeatherHandler) GetPressure24h(w http.ResponseWriter, r *http.Request) {
	s.GetGeneric24h(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiation24h(w http.ResponseWriter, r *http.Request) {
	s.GetGeneric24h(w, r, "solar_radiation")
}
