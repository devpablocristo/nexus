package actiontypes

import (
	"context"
	"errors"
	"net/http"
	"time"

	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	dto "github.com/devpablocristo/nexus/v3/review/internal/actiontypes/handler/dto"
	domain "github.com/devpablocristo/nexus/v3/review/internal/actiontypes/usecases/domain"
	"github.com/devpablocristo/nexus/v3/review/internal/shared"
	"github.com/google/uuid"
)

type actionTypeUsecase interface {
	Create(ctx context.Context, at domain.ActionType) (domain.ActionType, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.ActionType, error)
	List(ctx context.Context) ([]domain.ActionType, error)
	Update(ctx context.Context, at domain.ActionType) (domain.ActionType, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	uc actionTypeUsecase
}

func NewHandler(uc actionTypeUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/action-types", h.create)
	mux.HandleFunc("GET /v1/action-types", h.list)
	mux.HandleFunc("GET /v1/action-types/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/action-types/{id}", h.update)
	mux.HandleFunc("DELETE /v1/action-types/{id}", h.deleteByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body dto.CreateActionTypeRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.Name == "" {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "name is required")
		return
	}

	at := domain.ActionType{
		Name:               body.Name,
		Description:        body.Description,
		Category:           body.Category,
		RiskClass:          domain.RiskClass(body.RiskClass),
		Schema:             body.Schema,
		Reversible:         body.Reversible,
		RequiresBreakGlass: body.RequiresBreakGlass,
	}

	created, err := h.uc.Create(r.Context(), at)
	if err != nil {
		shared.WriteInternalError(w, err, "create action type")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusCreated, toResponse(created))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.List(r.Context())
	if err != nil {
		shared.WriteInternalError(w, err, "list action types")
		return
	}
	out := make([]dto.ActionTypeResponse, 0, len(list))
	for _, at := range list {
		out = append(out, toResponse(at))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	at, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toResponse(at))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	at, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var body dto.UpdateActionTypeRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.Name != nil {
		at.Name = *body.Name
	}
	if body.Description != nil {
		at.Description = *body.Description
	}
	if body.Category != nil {
		at.Category = *body.Category
	}
	if body.RiskClass != nil {
		at.RiskClass = domain.RiskClass(*body.RiskClass)
	}
	if body.Schema != nil {
		at.Schema = *body.Schema
	}
	if body.Reversible != nil {
		at.Reversible = *body.Reversible
	}
	if body.RequiresBreakGlass != nil {
		at.RequiresBreakGlass = *body.RequiresBreakGlass
	}
	if body.Enabled != nil {
		at.Enabled = *body.Enabled
	}

	updated, err := h.uc.Update(r.Context(), at)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toResponse(updated))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toResponse(at domain.ActionType) dto.ActionTypeResponse {
	return dto.ActionTypeResponse{
		ID:                 at.ID.String(),
		Name:               at.Name,
		Description:        at.Description,
		Category:           at.Category,
		RiskClass:          string(at.RiskClass),
		Schema:             at.Schema,
		Reversible:         at.Reversible,
		RequiresBreakGlass: at.RequiresBreakGlass,
		Enabled:            at.Enabled,
		CreatedAt:          at.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          at.UpdatedAt.Format(time.RFC3339),
	}
}

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "action type not found")
		return
	}
	shared.WriteInternalError(w, err, "action type operation failed")
}
