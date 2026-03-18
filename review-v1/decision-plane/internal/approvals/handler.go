package approvals

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	approvaldomain "github.com/devpablocristo/nexus/review-v1/internal/approvals/usecases/domain"
	approvaldto "github.com/devpablocristo/nexus/review-v1/internal/approvals/handler/dto"
	"github.com/devpablocristo/nexus/review-v1/internal/shared"
)

type approvalUsecase interface {
	ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error)
	Approve(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error
	Reject(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error
}

type Handler struct {
	uc approvalUsecase
}

func NewHandler(uc approvalUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/approvals/pending", h.listPending)
	mux.HandleFunc("POST /v1/approvals/{id}/approve", h.approve)
	mux.HandleFunc("POST /v1/approvals/{id}/reject", h.reject)
}

func (h *Handler) listPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.ListPending(r.Context(), shared.DefaultListLimit)
	if err != nil {
		shared.WriteInternalError(w, err, "list pending approvals failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body approvaldto.ApprovalDecisionRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.Approve(r.Context(), id, body.DecidedBy, body.Note); err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body approvaldto.ApprovalDecisionRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.Reject(r.Context(), id, body.DecidedBy, body.Note); err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func writeApprovalUsecaseError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotPending) {
		shared.WriteError(w, http.StatusConflict, "CONFLICT", "approval is not pending")
		return
	}
	if errors.Is(err, ErrNotFound) {
		shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "approval not found")
		return
	}
	shared.WriteInternalError(w, err, "approval operation failed")
}
