package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/alpineworks/ootel"
	"github.com/gorilla/mux"
	"github.com/michaelpeterswa/lfpweather-api/internal/config"
	"github.com/michaelpeterswa/lfpweather-api/internal/dragonfly"
	"github.com/michaelpeterswa/lfpweather-api/internal/handlers"
	"github.com/michaelpeterswa/lfpweather-api/internal/logging"
	"github.com/michaelpeterswa/lfpweather-api/internal/middleware"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

func main() {
	slogHandler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(slogHandler))

	slog.Info("welcome to lfpweather-api!")

	c, err := config.NewConfig()
	if err != nil {
		slog.Error("could not create config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slogLevel, err := logging.LogLevelToSlogLevel(c.LogLevel)
	if err != nil {
		slog.Error("could not parse log level", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(slogLevel)

	ctx := context.Background()

	ootelClient := ootel.NewOotelClient(
		ootel.WithMetricConfig(
			ootel.NewMetricConfig(
				c.MetricsEnabled,
				c.MetricsPort,
			),
		),
		ootel.WithTraceConfig(
			ootel.NewTraceConfig(
				c.TracingEnabled,
				c.TracingSampleRate,
				c.TracingService,
				c.TracingVersion,
			),
		),
	)

	shutdown, err := ootelClient.Init(ctx)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = shutdown(ctx)
	}()

	dragonflyClient, err := dragonfly.NewDragonflyClient(c.DragonflyHost, c.DragonflyPort, c.DragonflyAuth, c.CacheResultsDuration, c.DragonflyKeyPrefix)
	if err != nil {
		slog.Error("error initializing dragonfly client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	timescaleClient, err := timescale.NewTimescaleClient(ctx, c.TimescaleConnString, timescale.WithDragonflyClient(dragonflyClient))
	if err != nil {
		slog.Error("could not create timescale client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	weatherHandler := handlers.NewWeatherHandler(timescaleClient)

	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api").Subrouter()
	v1Subrouter := apiRouter.PathPrefix("/v1").Subrouter()

	// last data
	v1Subrouter.HandleFunc("/temperature/last", weatherHandler.GetTemperatureLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/last", weatherHandler.GetHumidityLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/last", weatherHandler.GetPressureLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/last", weatherHandler.GetSolarRadiationLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/wind_speed/last", weatherHandler.GetWindSpeedHighLast10MinLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/24h_rain/last", weatherHandler.GetRainLast24hLast).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/uv_index/last", weatherHandler.GetUVIndexLast).Methods(http.MethodGet)

	//12h data
	v1Subrouter.HandleFunc("/temperature/12h", weatherHandler.GetTemperature12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/12h", weatherHandler.GetHumidity12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/12h", weatherHandler.GetPressure12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/12h", weatherHandler.GetSolarRadiation12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/wind_speed/12h", weatherHandler.GetWindSpeedLast12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/rain_rate/12h", weatherHandler.GetRainRateLast12h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/uv_index/12h", weatherHandler.GetUVIndex12h).Methods(http.MethodGet)

	// 24h data
	v1Subrouter.HandleFunc("/temperature/24h", weatherHandler.GetTemperature24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/24h", weatherHandler.GetHumidity24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/24h", weatherHandler.GetPressure24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/24h", weatherHandler.GetSolarRadiation24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/wind_speed/24h", weatherHandler.GetWindSpeedLast24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/rain_rate/24h", weatherHandler.GetRainRateLast24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/uv_index/24h", weatherHandler.GetUVIndex24h).Methods(http.MethodGet)

	// 7d data
	v1Subrouter.HandleFunc("/temperature/7d", weatherHandler.GetTemperature7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/7d", weatherHandler.GetHumidity7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/7d", weatherHandler.GetPressure7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/7d", weatherHandler.GetSolarRadiation7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/wind_speed/7d", weatherHandler.GetWindSpeedLast7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/rain_rate/7d", weatherHandler.GetRainRateLast7d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/uv_index/7d", weatherHandler.GetUVIndex7d).Methods(http.MethodGet)

	// 30d data
	v1Subrouter.HandleFunc("/temperature/30d", weatherHandler.GetTemperature30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/30d", weatherHandler.GetHumidity30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/30d", weatherHandler.GetPressure30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/30d", weatherHandler.GetSolarRadiation30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/wind_speed/30d", weatherHandler.GetWindSpeedLast30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/rain_rate/30d", weatherHandler.GetRainRateLast30d).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/uv_index/30d", weatherHandler.GetUVIndex30d).Methods(http.MethodGet)

	if c.AuthenticationEnabled {
		authenticationMiddleware := middleware.NewAuthenticationMiddlewareClient(
			middleware.WithAPIKeys(c.APIKeys),
		)
		apiRouter.Use(authenticationMiddleware.AuthenticationMiddleware)
	}

	http.Handle("/", r)

	err = http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil)
	if err != nil {
		slog.Error("could not start http server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
