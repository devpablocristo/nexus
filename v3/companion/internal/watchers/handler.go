package watchers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	"github.com/devpablocristo/nexus/v3/companion/internal/watchers/handler/dto"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/watchers/usecases/domain"
)

// watcherUsecase port que el handler consume.
type watcherUsecase interface {
	Create(ctx context.Context, input CreateWatcherInput) (domain.Watcher, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Watcher, error)
	List(ctx context.Context, orgID string) ([]domain.Watcher, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateWatcherInput) (domain.Watcher, error)
	Delete(ctx context.Context, id uuid.UUID) error
	RunWatcher(ctx context.Context, watcherID uuid.UUID) (*domain.WatcherResult, error)
	ListProposals(ctx context.Context, watcherID uuid.UUID, limit int) ([]domain.Proposal, error)
}

// Handler es el handler HTTP del módulo watchers.
type Handler struct {
	uc watcherUsecase
}

// NewHandler crea un handler de watchers.
func NewHandler(uc watcherUsecase) *Handler {
	return &Handler{uc: uc}
}

// Register registra las rutas en un http.ServeMux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/watchers", h.create)
	mux.HandleFunc("GET /v1/watchers", h.list)
	mux.HandleFunc("GET /v1/watchers/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/watchers/{id}", h.update)
	mux.HandleFunc("DELETE /v1/watchers/{id}", h.remove)
	mux.HandleFunc("POST /v1/watchers/{id}/run", h.run)
	mux.HandleFunc("GET /v1/watchers/{id}/proposals", h.listProposals)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateWatcherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	result, err := h.uc.Create(r.Context(), CreateWatcherInput{
		OrgID:       req.OrgID,
		Name:        req.Name,
		WatcherType: domain.WatcherType(req.WatcherType),
		Config:      req.Config,
		Enabled:     req.Enabled,
	})
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not create watcher")
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusCreated, dto.WatcherToResponse(result))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "missing_org_id", "org_id query parameter required")
		return
	}

	watchers, err := h.uc.List(r.Context(), orgID)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not list watchers")
		return
	}

	items := make([]dto.WatcherResponse, 0, len(watchers))
	for _, w := range watchers {
		items = append(items, dto.WatcherToResponse(w))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, dto.WatcherListResponse{Watchers: items})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid watcher id")
		return
	}

	watcher, err := h.uc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sharedhandlers.WriteError(w, http.StatusNotFound, "not_found", "watcher not found")
			return
		}
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not get watcher")
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, dto.WatcherToResponse(watcher))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid watcher id")
		return
	}

	var req dto.UpdateWatcherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	input := UpdateWatcherInput{
		Name:    req.Name,
		Enabled: req.Enabled,
	}
	if req.Config != nil {
		input.Config = req.Config
	}

	result, err := h.uc.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sharedhandlers.WriteError(w, http.StatusNotFound, "not_found", "watcher not found")
			return
		}
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not update watcher")
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, dto.WatcherToResponse(result))
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid watcher id")
		return
	}

	if err := h.uc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			sharedhandlers.WriteError(w, http.StatusNotFound, "not_found", "watcher not found")
			return
		}
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not delete watcher")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid watcher id")
		return
	}

	result, err := h.uc.RunWatcher(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			sharedhandlers.WriteError(w, http.StatusNotFound, "not_found", "watcher not found")
			return
		}
		if errors.Is(err, ErrWatcherDisabled) {
			sharedhandlers.WriteError(w, http.StatusConflict, "watcher_disabled", "watcher is disabled")
			return
		}
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not run watcher")
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, dto.RunResultResponse{
		Found:    result.Found,
		Proposed: result.Proposed,
		Executed: result.Executed,
		Errors:   result.Errors,
	})
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid watcher id")
		return
	}

	proposals, err := h.uc.ListProposals(r.Context(), id, 50)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, "internal_error", "could not list proposals")
		return
	}

	items := make([]dto.ProposalResponse, 0, len(proposals))
	for _, p := range proposals {
		items = append(items, dto.ProposalToResponse(p))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, dto.ProposalListResponse{Proposals: items})
}
