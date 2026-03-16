package incidents

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	incidentdto "nexus/v2/control-workers/internal/incidents/handler/dto"
	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

type incidentUsecase interface {
	Create(ctx context.Context, req CreateRequest) (incidentdomain.Incident, error)
	List(ctx context.Context, req ListRequest) ([]incidentdomain.Incident, error)
	GetByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error)
	UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (incidentdomain.Incident, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error)
}

type Handler struct {
	uc incidentUsecase
}

func NewHandler(uc incidentUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/incidents", h.create)
	mux.HandleFunc("GET /v1/incidents", h.list)
	mux.HandleFunc("GET /v1/incidents/{id}", h.getByID)
	mux.HandleFunc("PATCH /v1/incidents/{id}", h.updateByID)
	mux.HandleFunc("DELETE /v1/incidents/{id}", h.deleteByID)
	mux.HandleFunc("POST /v1/incidents/{id}/archive", h.archiveByID)
	mux.HandleFunc("POST /v1/incidents/{id}/restore", h.restoreByID)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req incidentdto.CreateIncidentRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeIncidentError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	item, err := h.uc.Create(r.Context(), CreateRequest{
		SourceKind:   incidentdomain.SourceKind(req.SourceKind),
		SourceID:     req.SourceID,
		ActionType:   req.ActionType,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Trigger:      incidentdomain.Trigger(req.Trigger),
		RiskLevel:    incidentdomain.RiskLevel(req.RiskLevel),
		Summary:      req.Summary,
		Reason:       req.Reason,
		Details:      req.Details,
	})
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/v1/incidents/"+item.ID)
	sharedhandlers.WriteJSON(w, http.StatusCreated, toIncidentResponse(item))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := sharedhandlers.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeIncidentError(w, http.StatusBadRequest, "VALIDATION", "limit must be a positive integer")
		return
	}
	archived, err := sharedhandlers.ParseArchived(r.URL.Query().Get("archived"))
	if err != nil {
		writeIncidentError(w, http.StatusBadRequest, "VALIDATION", "archived must be true or false")
		return
	}

	items, err := h.uc.List(r.Context(), ListRequest{
		SourceKind: r.URL.Query().Get("source_kind"),
		ResourceID: r.URL.Query().Get("resource_id"),
		Trigger:    r.URL.Query().Get("trigger"),
		Severity:   r.URL.Query().Get("severity"),
		Status:     r.URL.Query().Get("status"),
		Archived:   archived,
		Limit:      limit,
	})
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}

	resp := incidentdto.ListIncidentsResponse{Items: make([]incidentdto.IncidentResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toIncidentResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIncidentID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toIncidentResponse(item))
}

func (h *Handler) updateByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIncidentID(w, r)
	if !ok {
		return
	}

	var req incidentdto.UpdateIncidentRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeIncidentError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	updateReq := UpdateRequest{Details: req.Details}
	if req.Status != nil {
		value := incidentdomain.Status(*req.Status)
		updateReq.Status = &value
	}
	updateReq.Summary = req.Summary
	updateReq.Reason = req.Reason

	item, err := h.uc.UpdateByID(r.Context(), id, updateReq)
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toIncidentResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIncidentID(w, r)
	if !ok {
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIncidentID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toIncidentResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIncidentID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writeIncidentUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toIncidentResponse(item))
}

func parseIncidentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeIncidentError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeIncidentUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeIncidentError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeIncidentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeIncidentError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, incidentdto.ErrorResponse{
		Error: incidentdto.ErrorObject{Code: code, Message: message},
	})
}

func toIncidentResponse(item incidentdomain.Incident) incidentdto.IncidentResponse {
	actionID := ""
	if item.SourceKind == incidentdomain.SourceKindAction {
		actionID = item.SourceID
	}
	return incidentdto.IncidentResponse{
		ID:           item.ID,
		SourceKind:   string(item.SourceKind),
		SourceID:     item.SourceID,
		ActionID:     actionID,
		ActionType:   item.ActionType,
		ResourceID:   item.ResourceID,
		ResourceType: item.ResourceType,
		Trigger:      string(item.Trigger),
		RiskLevel:    string(item.RiskLevel),
		Severity:     string(item.Severity),
		Status:       string(item.Status),
		Summary:      item.Summary,
		Reason:       item.Reason,
		Details:      cloneDetails(item.Details),
		Archived:     item.ArchivedAt != nil,
		ArchivedAt:   item.ArchivedAt,
		ResolvedAt:   item.ResolvedAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}
