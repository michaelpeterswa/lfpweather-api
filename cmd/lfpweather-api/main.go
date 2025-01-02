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
	v1Subrouter.HandleFunc("/temperature/24h", weatherHandler.GetTemperature24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/humidity/24h", weatherHandler.GetHumidity24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/pressure/24h", weatherHandler.GetPressure24h).Methods(http.MethodGet)
	v1Subrouter.HandleFunc("/solar_radiation/24h", weatherHandler.GetSolarRadiation24h).Methods(http.MethodGet)

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
