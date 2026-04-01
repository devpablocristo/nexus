package approvals

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/http/go/httpjson"
	approvaldto "github.com/devpablocristo/nexus/v3/review/internal/approvals/handler/dto"
	approvaldomain "github.com/devpablocristo/nexus/v3/review/internal/approvals/usecases/domain"
	"github.com/google/uuid"
	"github.com/devpablocristo/core/errors/go/domainerr"
)

const defaultListLimit = 50

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
	list, err := h.uc.ListPending(r.Context(), defaultListLimit)
	if err != nil {
		httpjson.WriteFlatInternalError(w, err, "list pending approvals failed")
		return
	}
	out := make([]approvaldto.ApprovalResponse, 0, len(list))
	for _, a := range list {
		out = append(out, toApprovalResponse(a))
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body approvaldto.ApprovalDecisionRequest
	if err := httpjson.DecodeJSON(r, &body); err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.Approve(r.Context(), id, decisionActorID(r, body.DecidedBy), body.Note); err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body approvaldto.ApprovalDecisionRequest
	if err := httpjson.DecodeJSON(r, &body); err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.Reject(r.Context(), id, decisionActorID(r, body.DecidedBy), body.Note); err != nil {
		writeApprovalUsecaseError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// toApprovalResponse convierte entidad de dominio a DTO HTTP.
func toApprovalResponse(a approvaldomain.Approval) approvaldto.ApprovalResponse {
	approveCount := 0
	for _, d := range a.Decisions {
		if d.Action == "approve" {
			approveCount++
		}
	}

	resp := approvaldto.ApprovalResponse{
		ID:                a.ID.String(),
		RequestID:         a.RequestID.String(),
		Status:            string(a.Status),
		DecidedBy:         a.DecidedBy,
		DecisionNote:      a.DecisionNote,
		ExpiresAt:         a.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		CreatedAt:         a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		BreakGlass:        a.BreakGlass,
		RequiredApprovals: a.RequiredApprovals,
		CurrentApprovals:  approveCount,
	}
	if a.DecidedAt != nil {
		s := a.DecidedAt.Format("2006-01-02T15:04:05Z")
		resp.DecidedAt = &s
	}
	for _, d := range a.Decisions {
		resp.Decisions = append(resp.Decisions, approvaldto.ApprovalDecisionDTO{
			ApproverID: d.ApproverID,
			Action:     d.Action,
			Note:       d.Note,
			DecidedAt:  d.DecidedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	return resp
}

func writeApprovalUsecaseError(w http.ResponseWriter, err error) {
	if domainerr.IsConflict(err) {
		// Usar el mensaje del error de dominio (distingue "not pending" vs "already decided")
		var de domainerr.Error
		msg := "conflict"
		if errors.As(err, &de) {
			msg = de.Message()
		}
		httpjson.WriteFlatError(w, http.StatusConflict, "CONFLICT", msg)
		return
	}
	if domainerr.IsNotFound(err) {
		httpjson.WriteFlatError(w, http.StatusNotFound, "NOT_FOUND", "approval not found")
		return
	}
	httpjson.WriteFlatInternalError(w, err, "approval operation failed")
}

func decisionActorID(r *http.Request, explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	return strings.TrimSpace(r.Header.Get("X-User-ID"))
}
