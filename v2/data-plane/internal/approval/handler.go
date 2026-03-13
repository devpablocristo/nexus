package approval

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	approvaldto "nexus/v2/data-plane/internal/approval/handler/dto"
	domain "nexus/v2/data-plane/internal/approval/usecases/domain"
)

type approvalUsecase interface {
	ListPending(ctx context.Context, limit int) ([]domain.PendingApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PendingApproval, error)
	Approve(ctx context.Context, id uuid.UUID, decidedBy string) (domain.PendingApproval, error)
	Reject(ctx context.Context, id uuid.UUID, decidedBy string) (domain.PendingApproval, error)
}

type Handler struct {
	uc approvalUsecase
}

func NewHandler(uc approvalUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/approvals", h.listPending)
	mux.HandleFunc("GET /v1/approvals/{id}", h.getByID)
	mux.HandleFunc("POST /v1/approvals/{id}/approve", h.approve)
	mux.HandleFunc("POST /v1/approvals/{id}/reject", h.reject)
}

func (h *Handler) listPending(w http.ResponseWriter, r *http.Request) {
	items, err := h.uc.ListPending(r.Context(), 100)
	if err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}

	resp := approvaldto.ListApprovalsResponse{Items: make([]approvaldto.ApprovalItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toApprovalDTO(item))
	}
	writeApprovalJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseApprovalID(w, r)
	if !ok {
		return
	}
	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}
	writeApprovalJSON(w, http.StatusOK, toApprovalDTO(item))
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, true)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, false)
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	id, ok := parseApprovalID(w, r)
	if !ok {
		return
	}

	var req approvaldto.DecideRequest
	if err := decodeApprovalJSON(r, &req); err != nil {
		writeApprovalError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	var (
		err    error
		status string
	)
	if approve {
		_, err = h.uc.Approve(r.Context(), id, req.DecidedBy)
		status = "approved"
	} else {
		_, err = h.uc.Reject(r.Context(), id, req.DecidedBy)
		status = "rejected"
	}
	if err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}

	writeApprovalJSON(w, http.StatusOK, approvaldto.DecideResponse{Status: status})
}

func parseApprovalID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeApprovalError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func decodeApprovalJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func toApprovalDTO(item domain.PendingApproval) approvaldto.ApprovalItem {
	return approvaldto.ApprovalItem{
		ID:        item.ID.String(),
		IntentID:  uuidToStringPtr(item.IntentID),
		RequestID: item.RequestID,
		ToolName:  item.ToolName,
		Reason:    item.Reason,
		Status:    string(item.Status),
		DecidedBy: item.DecidedBy,
		DecidedAt: item.DecidedAt,
		ExpiresAt: item.ExpiresAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func uuidToStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func writeApprovalUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeApprovalError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeApprovalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeApprovalError(w http.ResponseWriter, status int, code, message string) {
	writeApprovalJSON(w, status, approvaldto.ErrorResponse{
		Error: approvaldto.ErrorObject{
			Code:    code,
			Message: message,
		},
	})
}

func writeApprovalJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
