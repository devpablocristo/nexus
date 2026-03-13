package policy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	policydto "nexus/v2/data-plane/internal/policy/handler/dto"
	policydomain "nexus/v2/data-plane/internal/policy/usecases/domain"
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
	mux.HandleFunc("PATCH /v1/policies/{id}", h.patchByID)
	mux.HandleFunc("DELETE /v1/policies/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/policies/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/policies/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req policydto.CreatePolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writePolicyError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	created, err := h.uc.Create(r.Context(), CreateRequest{
		ToolName:           req.ToolName,
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

	writeJSON(w, http.StatusCreated, toPolicyResponse(created))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	includeArchived, err := parseOptionalBool(r.URL.Query().Get("include_archived"))
	if err != nil {
		writePolicyError(w, http.StatusBadRequest, "VALIDATION", "include_archived must be true or false")
		return
	}

	items, err := h.uc.List(r.Context(), ListRequest{
		ToolName:        r.URL.Query().Get("tool_name"),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}

	resp := policydto.ListPoliciesResponse{
		Items: make([]policydto.PolicyResponse, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toPolicyResponse(item))
	}

	writeJSON(w, http.StatusOK, resp)
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

	writeJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) patchByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}

	var req policydto.UpdatePolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writePolicyError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	item, err := h.uc.UpdateByID(r.Context(), id, PolicyPatch{
		ToolName:           req.ToolName,
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

	writeJSON(w, http.StatusOK, toPolicyResponse(item))
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

	writeJSON(w, http.StatusOK, toPolicyResponse(item))
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

	writeJSON(w, http.StatusOK, toPolicyResponse(item))
}

func parsePolicyID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writePolicyError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionalBool(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func toPolicyResponse(item policydomain.Policy) policydto.PolicyResponse {
	return policydto.PolicyResponse{
		ID:                 item.ID.String(),
		ToolName:           item.ToolName,
		Effect:             string(item.Effect),
		Priority:           item.Priority,
		Expression:         item.Expression,
		Reason:             item.Reason,
		RequireApproval:    item.RequireApproval,
		ApprovalTTLSeconds: item.ApprovalTTLSeconds,
		Enabled:            item.Enabled,
		Archived:           item.Archived,
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
	writeJSON(w, status, policydto.ErrorResponse{
		Error: policydto.ErrorObject{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
