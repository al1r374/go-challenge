// Package api exposes a REST interface for the Estimation Service.
//
// Why REST?
//   - USS (and any other producer) can POST events with a simple JSON body.
//   - Operators can GET counts with curl / monitoring tools — no IDL compile step.
//   - Easy to put behind an API gateway, rate-limit, and observe with standard
//     HTTP middleware. gRPC remains a drop-in later via the same service.Estimation interface.
//
// Segment modes are configured via environment (ES_SEGMENTS), not via HTTP.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ArmanCreativeSolutions/go-challenge/internal/config"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/logging"
	"github.com/ArmanCreativeSolutions/go-challenge/internal/service"
)

// Handler serves HTTP endpoints for ES.
type Handler struct {
	es service.Estimation
}

// NewHandler constructs an API handler.
func NewHandler(es service.Estimation) *Handler {
	return &Handler{es: es}
}

type addRequest struct {
	UserID  string `json:"user_id"`
	Segment string `json:"segment"`
}

type countResponse struct {
	Segment string      `json:"segment"`
	Count   int64       `json:"count"`
	Mode    config.Mode `json:"mode"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Routes registers all HTTP routes on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/events", h.Add)
	mux.HandleFunc("GET /v1/segments/{segment}/count", h.Count)
	mux.HandleFunc("GET /healthz", h.Health)
}

// Add accepts a (user_id, segment) pair from USS.
//
//	POST /v1/events
//	{"user_id":"u104010","segment":"sports"}
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Warn(r.Context(), logging.EventESAddRejected,
			"component", "api",
			"code", logging.CodeInvalidInput,
			"reason", "invalid JSON body",
		)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Segment = strings.TrimSpace(req.Segment)
	if req.UserID == "" || req.Segment == "" {
		logging.Warn(r.Context(), logging.EventESAddRejected,
			"component", "api",
			"code", logging.CodeInvalidInput,
			"reason", "missing user_id or segment",
		)
		writeError(w, http.StatusBadRequest, "user_id and segment are required")
		return
	}
	if err := h.es.Add(r.Context(), req.UserID, req.Segment); err != nil {
		// Service already emitted event=es.add_failed.
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// Count returns unique active users for a segment.
//
//	GET /v1/segments/{segment}/count
func (h *Handler) Count(w http.ResponseWriter, r *http.Request) {
	segment := strings.TrimSpace(r.PathValue("segment"))
	if segment == "" {
		logging.Warn(r.Context(), logging.EventESCountRejected,
			"component", "api",
			"code", logging.CodeInvalidInput,
			"reason", "missing segment",
		)
		writeError(w, http.StatusBadRequest, "segment is required")
		return
	}
	n, err := h.es.Count(r.Context(), segment)
	if err != nil {
		// Service already emitted event=es.count_failed.
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, countResponse{
		Segment: segment,
		Count:   n,
		Mode:    h.es.Mode(segment),
	})
}

// Health is a liveness probe.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
