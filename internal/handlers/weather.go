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

func (s *WeatherHandler) GetColumnGeneric(w http.ResponseWriter, r *http.Request, tp timescale.GetColumnTemplateParameters) {
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
			rfc9457.WithTitle("failed to marshal data"),
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
			rfc9457.WithTitle("failed to write data"),
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
	s.GetColumnGeneric(w, r, timescale.GetColumnTemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "12h",
		TimeBucket:       "30m",
	})
}

func (s *WeatherHandler) GetColumn24h(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetColumnGeneric(w, r, timescale.GetColumnTemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "24h",
		TimeBucket:       "1h",
	})
}

func (s *WeatherHandler) GetColumn7d(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetColumnGeneric(w, r, timescale.GetColumnTemplateParameters{
		ColumnName:       columnName,
		LookbackInterval: "7d",
		TimeBucket:       "6h",
	})
}

func (s *WeatherHandler) GetColumn30d(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetColumnGeneric(w, r, timescale.GetColumnTemplateParameters{
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

func (s *WeatherHandler) GetWindSpeedLast12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "wind_speed_last")
}

func (s *WeatherHandler) GetRainRateLast12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "rain_rate_last")
}

func (s *WeatherHandler) GetUVIndex12h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn12h(w, r, "uv_index")
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

func (s *WeatherHandler) GetWindSpeedLast24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "wind_speed_last")
}

func (s *WeatherHandler) GetRainRateLast24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "rain_rate_last")
}

func (s *WeatherHandler) GetUVIndex24h(w http.ResponseWriter, r *http.Request) {
	s.GetColumn24h(w, r, "uv_index")
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

func (s *WeatherHandler) GetWindSpeedLast7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "wind_speed_last")
}

func (s *WeatherHandler) GetRainRateLast7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "rain_rate_last")
}

func (s *WeatherHandler) GetUVIndex7d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn7d(w, r, "uv_index")
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

func (s *WeatherHandler) GetWindSpeedLast30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "wind_speed_last")
}

func (s *WeatherHandler) GetRainRateLast30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "rain_rate_last")
}

func (s *WeatherHandler) GetUVIndex30d(w http.ResponseWriter, r *http.Request) {
	s.GetColumn30d(w, r, "uv_index")
}

// -------------

func (s *WeatherHandler) GetColumnLastGeneric(w http.ResponseWriter, r *http.Request, tp timescale.GetColumnLastTemplateParameters) {
	temperatures, err := s.timescaleClient.GetColumnLast(r.Context(), tp)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get data"),
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
			rfc9457.WithTitle("failed to write data"),
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

func (s *WeatherHandler) GetColumnLast(w http.ResponseWriter, r *http.Request, columnName string) {
	s.GetColumnLastGeneric(w, r, timescale.GetColumnLastTemplateParameters{
		ColumnName: columnName,
	})
}

func (s *WeatherHandler) GetTemperatureLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "temperature")
}

func (s *WeatherHandler) GetHumidityLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "humidity")
}

func (s *WeatherHandler) GetPressureLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "barometer_sea_level")
}

func (s *WeatherHandler) GetSolarRadiationLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "solar_radiation")
}

func (s *WeatherHandler) GetWindSpeedHighLast10MinLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "wind_speed_high_last_10_min")
}

func (s *WeatherHandler) GetRainLast24hLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "rain_last_24_hour")
}

func (s *WeatherHandler) GetUVIndexLast(w http.ResponseWriter, r *http.Request) {
	s.GetColumnLast(w, r, "uv_index")
}
