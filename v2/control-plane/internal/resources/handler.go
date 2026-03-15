package resources

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	resourcedto "nexus/v2/control-plane/internal/resources/handler/dto"
	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
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
	uc resourceUsecase
}

func NewHandler(uc resourceUsecase) *Handler {
	return &Handler{uc: uc}
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
	})
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}

	w.Header().Set("Location", "/v1/resources/"+item.ID)
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

	item, err := h.uc.UpdateByID(r.Context(), id, updateReq)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func (h *Handler) deleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	if err := h.uc.DeleteByID(r.Context(), id); err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) archiveByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.ArchiveByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toResourceResponse(item))
}

func (h *Handler) restoreByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseResourceID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.RestoreByID(r.Context(), id)
	if err != nil {
		writeResourceUsecaseError(w, err)
		return
	}
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
		ArchivedAt:  item.ArchivedAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
