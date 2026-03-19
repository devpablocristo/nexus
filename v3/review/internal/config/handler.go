package config

import (
	"context"
	"encoding/json"
	"net/http"

	configdomain "github.com/devpablocristo/nexus/v3/review/internal/config/usecases/domain"
	"github.com/devpablocristo/nexus/v3/review/internal/shared"
)

type configUsecase interface {
	GetConfig(ctx context.Context) (*configdomain.SystemConfig, error)
	UpdateConfig(ctx context.Context, cfg configdomain.SystemConfig) (*configdomain.SystemConfig, error)
	ResetConfig(ctx context.Context) (*configdomain.SystemConfig, error)
	UpdateSection(ctx context.Context, section string, data json.RawMessage) (*configdomain.SystemConfig, error)
}

// Handler gestiona los endpoints de configuración
type Handler struct {
	uc configUsecase
}

func NewHandler(uc configUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/config", h.getConfig)
	mux.HandleFunc("PATCH /v1/config", h.updateConfig)
	mux.HandleFunc("PATCH /v1/config/{section}", h.updateSection)
	mux.HandleFunc("POST /v1/config/reset", h.resetConfig)
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.uc.GetConfig(r.Context())
	if err != nil {
		shared.WriteInternalError(w, err, "get config")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg configdomain.SystemConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid JSON body")
		return
	}
	updated, err := h.uc.UpdateConfig(r.Context(), cfg)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *Handler) updateSection(w http.ResponseWriter, r *http.Request) {
	section := r.PathValue("section")
	var data json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid JSON body")
		return
	}
	updated, err := h.uc.UpdateSection(r.Context(), section, data)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *Handler) resetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.uc.ResetConfig(r.Context())
	if err != nil {
		shared.WriteInternalError(w, err, "reset config")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}
