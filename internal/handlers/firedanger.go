package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"alpineworks.io/rfc9457"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type FireDangerHandler struct {
	timescaleClient *timescale.TimescaleClient
}

func NewFireDangerHandler(timescaleClient *timescale.TimescaleClient) *FireDangerHandler {
	return &FireDangerHandler{timescaleClient: timescaleClient}
}

// GetSummary serves GET /api/v1/fire_danger/summary: the latest reading plus a
// season-calibrated danger rating for the frontend fire weather section.
func (s *FireDangerHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.timescaleClient.GetFireDangerSummary(r.Context())
	if err != nil {
		writeFireDangerProblem(w, r, "failed to get fire danger summary", err.Error())
		return
	}

	res, err := json.Marshal(summary)
	if err != nil {
		writeFireDangerProblem(w, r, "failed to marshal fire danger summary", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		slog.Error("failed to write fire danger summary", slog.String("error", err.Error()))
	}
}

// writeFireDangerProblem writes an RFC 9457 problem+json error response.
func writeFireDangerProblem(w http.ResponseWriter, r *http.Request, title, detail string) {
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
