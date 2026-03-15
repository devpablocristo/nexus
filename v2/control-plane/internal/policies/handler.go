package policies

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	policydto "nexus/v2/control-plane/internal/policies/handler/dto"
	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
	"nexus/v2/control-plane/internal/shared/actors"
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

type policyAuditSink interface {
	Write(ctx context.Context, req sharedaudit.WriteRequest) error
}

type Handler struct {
	uc    policyUsecase
	audit policyAuditSink
}

func NewHandler(uc policyUsecase) *Handler { return &Handler{uc: uc} }

func (h *Handler) WithAuditSink(sink policyAuditSink) *Handler {
	h.audit = sink
	return h
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
	actor, ok := parsePolicyActor(w, r)
	if !ok {
		return
	}
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
	h.emitAudit(r.Context(), actor, "policy_created", item, map[string]any{
		"effect":           string(item.Effect),
		"require_approval": item.RequireApproval,
		"enabled":          item.Enabled,
	})
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
	actor, ok := parsePolicyActor(w, r)
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
	h.emitAudit(r.Context(), actor, "policy_updated", item, map[string]any{
		"effect":           string(item.Effect),
		"require_approval": item.RequireApproval,
		"enabled":          item.Enabled,
	})
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	actor, ok := parsePolicyActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "policy_deleted", item, map[string]any{
		"effect": string(item.Effect),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	actor, ok := parsePolicyActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "policy_archived", item, map[string]any{
		"archived_at": item.ArchivedAt,
	})
	sharedhandlers.WriteJSON(w, http.StatusOK, toPolicyResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyID(w, r)
	if !ok {
		return
	}
	actor, ok := parsePolicyActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writePolicyUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "policy_restored", item, map[string]any{
		"effect": string(item.Effect),
	})
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

func parsePolicyActor(w http.ResponseWriter, r *http.Request) (*sharedaudit.Actor, bool) {
	actor, err := actors.FromRequest(r)
	if err == nil {
		return actor, true
	}
	if errors.Is(err, actors.ErrIncompleteActorHeaders) {
		writePolicyError(w, http.StatusBadRequest, "VALIDATION", "actor headers require both X-Nexus-Actor-Type and X-Nexus-Actor-Id")
		return nil, false
	}
	writePolicyError(w, http.StatusBadRequest, "VALIDATION", err.Error())
	return nil, false
}

func (h *Handler) emitAudit(ctx context.Context, actor *sharedaudit.Actor, eventType string, item policydomain.Policy, data map[string]any) {
	if h.audit == nil {
		return
	}
	if err := h.audit.Write(ctx, sharedaudit.WriteRequest{
		EventType:     eventType,
		SourceService: "control-plane",
		ResourceType:  item.ResourceType,
		Actor:         actor,
		Summary:       policyAuditSummary(eventType),
		Data:          clonePolicyMap(data, item),
		OccurredAt:    item.UpdatedAt,
	}); err != nil {
		log.Printf("control-plane policy audit failed: policy_id=%s event_type=%s err=%v", item.ID, eventType, err)
	}
}

func policyAuditSummary(eventType string) string {
	switch eventType {
	case "policy_created":
		return "policy created"
	case "policy_updated":
		return "policy updated"
	case "policy_deleted":
		return "policy deleted"
	case "policy_archived":
		return "policy archived"
	case "policy_restored":
		return "policy restored"
	default:
		return "policy changed"
	}
}

func clonePolicyMap(data map[string]any, item policydomain.Policy) map[string]any {
	out := make(map[string]any, len(data)+4)
	for key, value := range data {
		out[key] = value
	}
	out["policy_id"] = item.ID
	out["action_type"] = item.ActionType
	out["resource_type"] = item.ResourceType
	out["priority"] = item.Priority
	return out
}
