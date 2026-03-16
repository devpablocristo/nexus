package resources

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
	resourcedto "nexus/v2/control-plane/internal/resources/handler/dto"
	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
	"nexus/v2/control-plane/internal/shared/actors"
)

type resourceUsecase interface {
	Create(ctx context.Context, req CreateRequest) (resourcedomain.ProtectedResource, error)
	List(ctx context.Context, req ListRequest) ([]resourcedomain.ProtectedResource, error)
	GetByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error)
	UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (resourcedomain.ProtectedResource, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error)
}

type Handler struct {
	uc    resourceUsecase
	audit resourceAuditSink
}

func NewHandler(uc resourceUsecase) *Handler {
	return &Handler{uc: uc}
}

type resourceAuditSink interface {
	Write(ctx context.Context, req sharedaudit.WriteRequest) error
}

func (h *Handler) WithAuditSink(sink resourceAuditSink) *Handler {
	h.audit = sink
	return h
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/resources", h.create)
	mux.HandleFunc("GET /v1/resources", h.list)
	mux.HandleFunc("GET /v1/resources/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/resources/{id}", h.updateByID)
	mux.HandleFunc("DELETE /v1/resources/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/resources/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/resources/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, ok := parseAdminActor(w, r)
	if !ok {
		return
	}
	var req resourcedto.CreateResourceRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeResourceError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	item, err := h.uc.Create(r.Context(), CreateRequest{
		Type:        resourcedomain.ResourceType(req.Type),
		Name:        req.Name,
		Environment: req.Environment,
		Chain:       req.Chain,
		Labels:      req.Labels,
		Criticality: resourcedomain.Criticality(req.Criticality),
		IsCanary:    req.IsCanary,
	})
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/v1/resources/"+item.ID)
	h.emitAudit(r.Context(), actor, "resource_created", item, map[string]any{
		"name":        item.Name,
		"environment": item.Environment,
		"chain":       item.Chain,
	})
	sharedhandlers.WriteJSON(w, http.StatusCreated, toResourceResponse(item))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := sharedhandlers.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeResourceError(w, http.StatusBadRequest, "VALIDATION", "limit must be a positive integer")
		return
	}
	archived, err := sharedhandlers.ParseArchived(r.URL.Query().Get("archived"))
	if err != nil {
		writeResourceError(w, http.StatusBadRequest, "VALIDATION", "archived must be true or false")
		return
	}

	items, err := h.uc.List(r.Context(), ListRequest{
		Type:        r.URL.Query().Get("type"),
		Environment: r.URL.Query().Get("environment"),
		Archived:    archived,
		Limit:       limit,
	})
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}

	resp := resourcedto.ListResourcesResponse{Items: make([]resourcedto.ResourceResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toResourceResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func (h *Handler) updateByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	actor, ok := parseAdminActor(w, r)
	if !ok {
		return
	}

	var req resourcedto.UpdateResourceRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeResourceError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	updateReq := UpdateRequest{Labels: req.Labels}
	if req.Type != nil {
		value := resourcedomain.ResourceType(*req.Type)
		updateReq.Type = &value
	}
	if req.Name != nil {
		updateReq.Name = req.Name
	}
	if req.Environment != nil {
		updateReq.Environment = req.Environment
	}
	if req.Chain != nil {
		updateReq.Chain = req.Chain
	}
	if req.Criticality != nil {
		value := resourcedomain.Criticality(*req.Criticality)
		updateReq.Criticality = &value
	}
	if req.IsCanary != nil {
		updateReq.IsCanary = req.IsCanary
	}

	item, err := h.uc.UpdateByID(r.Context(), id, updateReq)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "resource_updated", item, map[string]any{
		"name":        item.Name,
		"environment": item.Environment,
		"chain":       item.Chain,
	})
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	actor, ok := parseAdminActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "resource_deleted", item, map[string]any{
		"name": item.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	actor, ok := parseAdminActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "resource_archived", item, map[string]any{
		"archived_at": item.ArchivedAt,
	})
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	actor, ok := parseAdminActor(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	h.emitAudit(r.Context(), actor, "resource_restored", item, map[string]any{
		"name": item.Name,
	})
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func parseResourceID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeResourceError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeResourceUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeResourceError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeResourceError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeResourceError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, resourcedto.ErrorResponse{
		Error: resourcedto.ErrorObject{Code: code, Message: message},
	})
}

func toResourceResponse(item resourcedomain.ProtectedResource) resourcedto.ResourceResponse {
	return resourcedto.ResourceResponse{
		ID:          item.ID,
		Type:        string(item.Type),
		Name:        item.Name,
		Environment: item.Environment,
		Chain:       item.Chain,
		Labels:      cloneLabels(item.Labels),
		Criticality: string(item.Criticality),
		IsCanary:    item.IsCanary,
		ArchivedAt:  item.ArchivedAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func parseAdminActor(w http.ResponseWriter, r *http.Request) (*sharedaudit.Actor, bool) {
	actor, err := actors.FromRequest(r)
	if err == nil {
		return actor, true
	}
	if errors.Is(err, actors.ErrIncompleteActorHeaders) {
		writeResourceError(w, http.StatusBadRequest, "VALIDATION", "actor headers require both X-Nexus-Actor-Type and X-Nexus-Actor-Id")
		return nil, false
	}
	writeResourceError(w, http.StatusBadRequest, "VALIDATION", err.Error())
	return nil, false
}

func (h *Handler) emitAudit(ctx context.Context, actor *sharedaudit.Actor, eventType string, item resourcedomain.ProtectedResource, data map[string]any) {
	if h.audit == nil {
		return
	}
	if err := h.audit.Write(ctx, sharedaudit.WriteRequest{
		EventType:     eventType,
		SourceService: "control-plane",
		ResourceID:    item.ID,
		ResourceType:  string(item.Type),
		Actor:         actor,
		Summary:       resourceAuditSummary(eventType),
		Data:          cloneMap(data),
		OccurredAt:    nowUTC(),
	}); err != nil {
		sharedobservability.LoggerFromContext(ctx).Error(
			"control-plane resource audit failed",
			"resource_id", item.ID,
			"event_type", eventType,
			"error", err,
		)
	}
}

func resourceAuditSummary(eventType string) string {
	switch eventType {
	case "resource_created":
		return "resource created"
	case "resource_updated":
		return "resource updated"
	case "resource_deleted":
		return "resource deleted"
	case "resource_archived":
		return "resource archived"
	case "resource_restored":
		return "resource restored"
	default:
		return "resource changed"
	}
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
