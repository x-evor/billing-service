package httpapi

import (
	"encoding/json"
	"net/http"

	"billing-service/internal/model"
	"billing-service/internal/service"
)

type Handler struct {
	service *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", h.ping)
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/v1/status", h.status)
	mux.HandleFunc("/v1/jobs/collect-and-rate", h.collectAndRate)
	mux.HandleFunc("/v1/jobs/reconcile", h.reconcile)
	return mux
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Ping())
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	ok, message := h.service.Health()
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"status":  map[bool]string{true: "ok", false: "degraded"}[ok],
		"message": message,
	})
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.service.Status())
}

func (h *Handler) collectAndRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := h.service.RunCollectAndRate(r.Context(), "collect-and-rate")
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := h.service.RunCollectAndRate(r.Context(), "reconcile")
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

var _ = model.JobResult{}
