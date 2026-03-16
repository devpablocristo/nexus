package audit

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	auditdto "nexus/v2/control-plane/internal/audit/handler/dto"
	auditdomain "nexus/v2/control-plane/internal/audit/usecases/domain"
)

type auditUsecase interface {
	Create(ctx context.Context, req sharedaudit.WriteRequest) (auditdomain.AuditRecord, error)
	List(ctx context.Context, req ListRequest) ([]auditdomain.AuditRecord, error)
	GetByID(ctx context.Context, id uuid.UUID) (auditdomain.AuditRecord, error)
}

type Handler struct {
	uc auditUsecase
}

func NewHandler(uc auditUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/audit", h.createInternal)
	mux.HandleFunc("GET /v1/audit", h.list)
	mux.HandleFunc("GET /v1/audit/{id}", h.getByID)
}

func (h *Handler) createInternal(w http.ResponseWriter, r *http.Request) {
	var req sharedaudit.WriteRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeAuditError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}
	item, err := h.uc.Create(r.Context(), req)
	if err != nil {
		writeAuditUsecaseError(w, err)
		return
	}
	w.Header().Set("Location", "/v1/audit/"+item.ID)
	sharedhandlers.WriteJSON(w, http.StatusCreated, toAuditResponse(item))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := sharedhandlers.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeAuditError(w, http.StatusBadRequest, "VALIDATION", "limit must be a positive integer")
		return
	}
	from, err := parseRFC3339(r.URL.Query().Get("from"))
	if err != nil {
		writeAuditError(w, http.StatusBadRequest, "VALIDATION", "from must be RFC3339")
		return
	}
	to, err := parseRFC3339(r.URL.Query().Get("to"))
	if err != nil {
		writeAuditError(w, http.StatusBadRequest, "VALIDATION", "to must be RFC3339")
		return
	}
	items, err := h.uc.List(r.Context(), ListRequest{
		ActionID:   r.URL.Query().Get("action_id"),
		IncidentID: r.URL.Query().Get("incident_id"),
		AlertID:    r.URL.Query().Get("alert_id"),
		ResourceID: r.URL.Query().Get("resource_id"),
		ActorID:    r.URL.Query().Get("actor_id"),
		EventType:  r.URL.Query().Get("event_type"),
		From:       from,
		To:         to,
		Limit:      limit,
	})
	if err != nil {
		writeAuditUsecaseError(w, err)
		return
	}
	resp := auditdto.ListAuditResponse{Items: make([]auditdto.AuditResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toAuditResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAuditID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeAuditUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toAuditResponse(item))
}

func parseAuditID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeAuditError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func parseRFC3339(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func writeAuditUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeAuditError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeAuditError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeAuditError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, auditdto.ErrorResponse{
		Error: auditdto.ErrorObject{Code: code, Message: message},
	})
}

func toAuditResponse(item auditdomain.AuditRecord) auditdto.AuditResponse {
	return auditdto.AuditResponse{
		ID:            item.ID,
		EventType:     item.EventType,
		SourceService: item.SourceService,
		ActionID:      item.ActionID,
		IncidentID:    item.IncidentID,
		AlertID:       item.AlertID,
		ResourceID:    item.ResourceID,
		ResourceType:  item.ResourceType,
		Actor:         item.Actor,
		Summary:       item.Summary,
		Data:          cloneData(item.Data),
		OccurredAt:    item.OccurredAt,
		CreatedAt:     item.CreatedAt,
	}
}
