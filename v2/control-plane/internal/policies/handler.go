package policies

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	policydto "nexus/v2/control-plane/internal/policies/handler/dto"
	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
)

type policyUsecase interface {
	Create(ctx context.Context, req CreateRequest) (policydomain.Policy, error)
	List(ctx context.Context, req ListRequest) ([]policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	UpdateByID(ctx context.Context, id uuid.UUID, patch PolicyPatch) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
}

type Handler struct{ uc policyUsecase }

func NewHandler(uc policyUsecase) *Handler { return &Handler{uc: uc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/policies", h.create)
	mux.HandleFunc("GET /v1/policies", h.list)
	mux.HandleFunc("GET /v1/policies/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/policies/{id}", h.patchByID)
	mux.HandleFunc("DELETE /v1/policies/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/policies/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/policies/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req policydto.CreatePolicyRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writePolicyError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}
	item, err := h.uc.Create(r.Context(), CreateRequest{
		ActionType:         req.ActionType,
		ResourceType:       req.ResourceType,
		Effect:             req.Effect,
		Priority:           req.Priority,
		Expression:         req.Expression,
		Reason:             req.Reason,
		RequireApproval:    req.RequireApproval,
		ApprovalTTLSeconds: req.ApprovalTTLSeconds,
		Enabled:            req.Enabled,
	})
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/policies/"+item.ID)
	sharedhandlers.WriteJSON(w, http.StatusCreated, toPolicyResponse(item))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	archived, err := sharedhandlers.ParseArchived(r.URL.Query().Get("archived"))
	if err != nil {
		writePolicyError(w, http.StatusBadRequest, "VALIDATION", "archived must be true or false")
		return
	}
	items, err := h.uc.List(r.Context(), ListRequest{
		ActionType:   r.URL.Query().Get("action_type"),
		ResourceType: r.URL.Query().Get("resource_type"),
		Archived:     archived,
	})
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	resp := policydto.ListPoliciesResponse{Items: make([]policydto.PolicyResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toPolicyResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) patchByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	var req policydto.UpdatePolicyRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writePolicyError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}
	item, err := h.uc.UpdateByID(r.Context(), id, PolicyPatch{
		ActionType:         req.ActionType,
		ResourceType:       req.ResourceType,
		Effect:             req.Effect,
		Priority:           req.Priority,
		Expression:         req.Expression,
		Reason:             req.Reason,
		RequireApproval:    req.RequireApproval,
		ApprovalTTLSeconds: req.ApprovalTTLSeconds,
		Enabled:            req.Enabled,
	})
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func parsePolicyID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writePolicyError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func toPolicyResponse(item policydomain.Policy) policydto.PolicyResponse {
	return policydto.PolicyResponse{
		ID:                 item.ID,
		ActionType:         item.ActionType,
		ResourceType:       item.ResourceType,
		Effect:             string(item.Effect),
		Priority:           item.Priority,
		Expression:         item.Expression,
		Reason:             item.Reason,
		RequireApproval:    item.RequireApproval,
		ApprovalTTLSeconds: item.ApprovalTTLSeconds,
		Enabled:            item.Enabled,
		Archived:           item.ArchivedAt != nil,
		ArchivedAt:         item.ArchivedAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func writePolicyUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writePolicyError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writePolicyError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writePolicyError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, policydto.ErrorResponse{
		Error: policydto.ErrorObject{Code: code, Message: message},
	})
}
