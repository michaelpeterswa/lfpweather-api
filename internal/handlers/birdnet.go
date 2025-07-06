package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"alpineworks.io/rfc9457"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type BirdnetHandler struct {
	timescaleClient *timescale.TimescaleClient
}

func NewBirdnetHandler(client *timescale.TimescaleClient) *BirdnetHandler {
	return &BirdnetHandler{
		timescaleClient: client,
	}
}

func (bh *BirdnetHandler) GetBirdCount24h(w http.ResponseWriter, r *http.Request) {
	birds, err := bh.timescaleClient.GetBirdnet(r.Context(), timescale.GetBirdnetTemplateParameters{
		LookbackInterval: "24h",
	})

	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to get 24h data"),
			rfc9457.WithDetail(fmt.Sprintf("error getting bird data: %s", err.Error())),
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

	res, err := json.Marshal(birds)
	if err != nil {
		statusCode := http.StatusInternalServerError

		problem := rfc9457.NewRFC9457(
			rfc9457.WithTitle("failed to marshal data"),
			rfc9457.WithDetail(fmt.Sprintf("error marshalling bird data: %s", err.Error())),
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
			rfc9457.WithDetail(fmt.Sprintf("error writing bird data: %s", err.Error())),
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
