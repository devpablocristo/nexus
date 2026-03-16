package alerts

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	alertdto "nexus/v2/control-workers/internal/alerts/handler/dto"
	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

type alertUsecase interface {
	Create(ctx context.Context, req CreateRequest) (alertdomain.Alert, error)
	List(ctx context.Context, req ListRequest) ([]alertdomain.Alert, error)
	GetByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error)
	UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (alertdomain.Alert, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error)
}

type Handler struct {
	uc alertUsecase
}

func NewHandler(uc alertUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/alerts", h.create)
	mux.HandleFunc("GET /v1/alerts", h.list)
	mux.HandleFunc("GET /v1/alerts/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/alerts/{id}", h.updateByID)
	mux.HandleFunc("DELETE /v1/alerts/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/alerts/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/alerts/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req alertdto.CreateAlertRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeAlertError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	status := alertdomain.Status(req.Status)
	item, err := h.uc.Create(r.Context(), CreateRequest{
		SourceKind: alertdomain.SourceKind(req.SourceKind),
		SourceID:   req.SourceID,
		ActionID:   req.ActionID,
		ResourceID: req.ResourceID,
		ResourceType: req.ResourceType,
		Channel:    alertdomain.Channel(req.Channel),
		Route:      req.Route,
		Severity:   alertdomain.Severity(req.Severity),
		Status:     status,
		Summary:    req.Summary,
		Body:       req.Body,
		Details:    req.Details,
	})
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/v1/alerts/"+item.ID)
	sharedhandlers.WriteJSON(w, http.StatusCreated, toAlertResponse(item))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := sharedhandlers.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeAlertError(w, http.StatusBadRequest, "VALIDATION", "limit must be a positive integer")
		return
	}
	archived, err := sharedhandlers.ParseArchived(r.URL.Query().Get("archived"))
	if err != nil {
		writeAlertError(w, http.StatusBadRequest, "VALIDATION", "archived must be true or false")
		return
	}

	items, err := h.uc.List(r.Context(), ListRequest{
		SourceKind: r.URL.Query().Get("source_kind"),
		Channel:    r.URL.Query().Get("channel"),
		Severity:   r.URL.Query().Get("severity"),
		Status:     r.URL.Query().Get("status"),
		Archived:   archived,
		Limit:      limit,
	})
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}

	resp := alertdto.ListAlertsResponse{Items: make([]alertdto.AlertResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toAlertResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAlertID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toAlertResponse(item))
}

func (h *Handler) updateByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAlertID(w, r)
	if !ok {
		return
	}

	var req alertdto.UpdateAlertRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeAlertError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	updateReq := UpdateRequest{Details: req.Details}
	if req.Status != nil {
		value := alertdomain.Status(*req.Status)
		updateReq.Status = &value
	}
	updateReq.Summary = req.Summary
	updateReq.Body = req.Body

	item, err := h.uc.UpdateByID(r.Context(), id, updateReq)
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toAlertResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAlertID(w, r)
	if !ok {
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeAlertUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAlertID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toAlertResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAlertID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writeAlertUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toAlertResponse(item))
}

func parseAlertID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeAlertError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeAlertUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeAlertError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeAlertError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeAlertError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, alertdto.ErrorResponse{
		Error: alertdto.ErrorObject{Code: code, Message: message},
	})
}

func toAlertResponse(item alertdomain.Alert) alertdto.AlertResponse {
	incidentID := ""
	if item.SourceKind == alertdomain.SourceKindIncident {
		incidentID = item.SourceID
	}
	return alertdto.AlertResponse{
		ID:           item.ID,
		SourceKind:   string(item.SourceKind),
		SourceID:     item.SourceID,
		IncidentID:   incidentID,
		ActionID:     detailString(item.Details, "action_id"),
		ResourceID:   detailString(item.Details, "resource_id"),
		ResourceType: detailString(item.Details, "resource_type"),
		Channel:      string(item.Channel),
		Route:        item.Route,
		Severity:     string(item.Severity),
		Status:       string(item.Status),
		Summary:      item.Summary,
		Body:         item.Body,
		Details:      cloneDetails(item.Details),
		Archived:     item.ArchivedAt != nil,
		ArchivedAt:   item.ArchivedAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func detailString(details map[string]any, key string) string {
	if len(details) == 0 {
		return ""
	}
	raw, ok := details[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}
