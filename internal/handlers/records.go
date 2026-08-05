package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/michaelpeterswa/lfpweather-api/internal/timescale"
)

type RecordsHandler struct {
	timescaleClient *timescale.TimescaleClient
	catalog         timescale.Catalog
	timeout         time.Duration
}

func NewRecordsHandler(client *timescale.TimescaleClient, catalog timescale.Catalog, timeout time.Duration) *RecordsHandler {
	return &RecordsHandler{
		timescaleClient: client,
		catalog:         catalog,
		timeout:         timeout,
	}
}

// GetRecords handles GET /api/v1/records/{period}: the record high and low of
// the curated metrics over a day, week, month, year, or all time. The optional
// ?at= query parameter selects a past period. A completed period is served with
// a long, immutable cache header because it can no longer change.
func (h *RecordsHandler) GetRecords(w http.ResponseWriter, r *http.Request) {
	period := mux.Vars(r)["period"]

	now := time.Now().UTC()
	at := now
	if raw := r.URL.Query().Get("at"); raw != "" {
		parsed, err := timescale.ParseAt(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid at parameter", err.Error())
			return
		}
		at = parsed
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp, err := h.timescaleClient.GetRecords(ctx, h.catalog, period, at, now)
	if err != nil {
		var ve *timescale.QueryValidationError
		if errors.As(err, &ve) {
			writeProblem(w, r, http.StatusBadRequest, "invalid records request", ve.Error())
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "failed to get records", err.Error())
		return
	}

	res, err := json.Marshal(resp)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "failed to marshal records", err.Error())
		return
	}

	// A completed period never changes, so it can be cached indefinitely; the
	// in-progress period gets a short cache lifetime.
	if resp.Complete {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		slog.Error("failed to write response", slog.String("error", err.Error()))
	}
}
