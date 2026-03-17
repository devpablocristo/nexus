package policies

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	policydomain "github.com/devpablocristo/nexus/review-v1/internal/policies/usecases/domain"
	policydto "github.com/devpablocristo/nexus/review-v1/internal/policies/handler/dto"
)

// Port mínimo: solo lo que el handler necesita
type policyUsecase interface {
	Create(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error)
	Update(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) error
	RestoreByID(ctx context.Context, id uuid.UUID) error
}

type Handler struct {
	uc policyUsecase
}

func NewHandler(uc policyUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/policies", h.create)
	mux.HandleFunc("GET /v1/policies", h.list)
	mux.HandleFunc("GET /v1/policies/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/policies/{id}", h.update)
	mux.HandleFunc("DELETE /v1/policies/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/policies/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/policies/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body policydto.CreatePolicyRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.Name == "" || body.Expression == "" || body.Effect == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "name, expression and effect are required")
		return
	}
	p := policydomain.Policy{
		Name:         body.Name,
		Description:  body.Description,
		Expression:   body.Expression,
		Effect:       body.Effect,
		RiskOverride: body.RiskOverride,
		Priority:     body.Priority,
		Enabled:      body.Enabled,
		ActionType:   body.ActionType,
		TargetSystem: body.TargetSystem,
		Origin:       "manual",
	}
	created, err := h.uc.Create(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusCreated, toPolicyResponse(created))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	archived := r.URL.Query().Get("archived") == "true"
	list, err := h.uc.List(r.Context(), ListFilters{IncludeArchived: archived})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	out := make([]policydto.PolicyResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toPolicyResponse(p))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	p, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(p))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	p, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	var body policydto.UpdatePolicyRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	// Aplicar solo los campos presentes (patch parcial)
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Expression != nil {
		p.Expression = *body.Expression
	}
	if body.Effect != nil {
		p.Effect = *body.Effect
	}
	if body.RiskOverride != nil {
		p.RiskOverride = body.RiskOverride
	}
	if body.Priority != nil {
		p.Priority = *body.Priority
	}
	if body.Enabled != nil {
		p.Enabled = *body.Enabled
	}
	if body.ActionType != nil {
		p.ActionType = body.ActionType
	}
	if body.TargetSystem != nil {
		p.TargetSystem = body.TargetSystem
	}
	updated, err := h.uc.Update(r.Context(), p)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(updated))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	if err := h.uc.ArchiveByID(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	if err := h.uc.RestoreByID(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

func toPolicyResponse(p policydomain.Policy) policydto.PolicyResponse {
	resp := policydto.PolicyResponse{
		ID:           p.ID.String(),
		Name:         p.Name,
		Description:  p.Description,
		Expression:   p.Expression,
		Effect:       p.Effect,
		RiskOverride: p.RiskOverride,
		Priority:     p.Priority,
		Origin:       p.Origin,
		Enabled:      p.Enabled,
		ActionType:   p.ActionType,
		TargetSystem: p.TargetSystem,
		CreatedAt:    p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
	}
	if p.ArchivedAt != nil {
		s := p.ArchivedAt.Format(time.RFC3339)
		resp.ArchivedAt = &s
	}
	return resp
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeUsecaseError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "policy not found")
		return
	}
	if errors.Is(err, ErrArchived) {
		writeError(w, http.StatusConflict, "CONFLICT", "policy is archived")
		return
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
}
