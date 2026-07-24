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

type QueryHandler struct {
	timescaleClient *timescale.TimescaleClient
	limits          timescale.QueryLimits
	timeout         time.Duration
}

func NewQueryHandler(client *timescale.TimescaleClient, limits timescale.QueryLimits, timeout time.Duration) *QueryHandler {
	return &QueryHandler{
		timescaleClient: client,
		limits:          limits,
		timeout:         timeout,
	}
}

// PostQuery handles POST /api/v1/query: a guarded, structured query over the
// sensor data with a caller-chosen metric, time range, and (optional) bucket.
func (h *QueryHandler) PostQuery(w http.ResponseWriter, r *http.Request) {
	var req timescale.QueryRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	plan, err := timescale.PlanQuery(req, h.limits, time.Now().UTC())
	if err != nil {
		var ve *timescale.QueryValidationError
		if errors.As(err, &ve) {
			writeProblem(w, r, http.StatusBadRequest, "invalid query", ve.Error())
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "failed to plan query", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp, err := h.timescaleClient.RunQuery(ctx, plan)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "failed to run query", err.Error())
		return
	}

	res, err := json.Marshal(resp)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "failed to marshal result", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(res); err != nil {
		slog.Error("failed to write response", slog.String("error", err.Error()))
	}
}
