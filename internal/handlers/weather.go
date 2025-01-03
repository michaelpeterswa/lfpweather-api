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

func (s *WeatherHandler) GetGeneric(w http.ResponseWriter, r *http.Request, tp timescale.TemplateParameters) {
	temperatures, err := s.timescaleClient.GetColumn(r.Context(), tp)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error getting data for column %s: %s", tp.ColumnName, err.Error())),
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
		}
		return
	}

	res, err := json.Marshal(temperatures)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to marshal 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error marshalling data for column %s: %s", tp.ColumnName, err.Error())),
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
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(res)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to write 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error writing data for column %s: %s", tp.ColumnName, err.Error())),
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
		}
		return
	}
}

func (s *WeatherHandler) GetColumn12h(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetGeneric(w, r, timescale.TemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "12h",
		TimeBucket:       "30m",
	})
}

func (s *WeatherHandler) GetColumn24h(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetGeneric(w, r, timescale.TemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "24h",
		TimeBucket:       "1h",
	})
}

func (s *WeatherHandler) GetColumn7d(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetGeneric(w, r, timescale.TemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "7d",
		TimeBucket:       "6h",
	})
}

func (s *WeatherHandler) GetColumn30d(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetGeneric(w, r, timescale.TemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "30d",
		TimeBucket:       "1d",
	})
}

// 12h

func (s *WeatherHandler) GetTemperature12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidity12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "humidity")
}

func (s *WeatherHandler) GetPressure12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiation12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "solar_radiation")
}

// 24h

func (s *WeatherHandler) GetTemperature24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidity24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "humidity")
}

func (s *WeatherHandler) GetPressure24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiation24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "solar_radiation")
}

// 7d

func (s *WeatherHandler) GetTemperature7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidity7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "humidity")
}

func (s *WeatherHandler) GetPressure7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiation7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "solar_radiation")
}

// 30d

func (s *WeatherHandler) GetTemperature30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidity30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "humidity")
}

func (s *WeatherHandler) GetPressure30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiation30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "solar_radiation")
}
