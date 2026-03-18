package learning

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	learningdto "github.com/devpablocristo/nexus/review-v1/internal/learning/handler/dto"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
	"github.com/devpablocristo/nexus/review-v1/internal/shared"
)

type learningUsecase interface {
	ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error)
	GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error)
	AcceptProposal(ctx context.Context, id uuid.UUID, decidedBy string) (*uuid.UUID, error)
	DismissProposal(ctx context.Context, id uuid.UUID, decidedBy string) error
	AnalyzeAndPropose(ctx context.Context) (int, error)
}

type Handler struct {
	uc learningUsecase
}

func NewHandler(uc learningUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/learning/proposals", h.listProposals)
	mux.HandleFunc("GET /v1/learning/proposals/{id}", h.getProposal)
	mux.HandleFunc("POST /v1/learning/proposals/{id}/accept", h.accept)
	mux.HandleFunc("POST /v1/learning/proposals/{id}/dismiss", h.dismiss)
	mux.HandleFunc("POST /v1/learning/analyze", h.analyze)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.ListPendingProposals(r.Context(), shared.DefaultListLimit)
	if err != nil {
		shared.WriteInternalError(w, err, "list proposals failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	p, err := h.uc.GetProposalByID(r.Context(), id)
	if err != nil {
		writeLearningUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body learningdto.ProposalDecisionRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	policyID, err := h.uc.AcceptProposal(r.Context(), id, body.DecidedBy)
	if err != nil {
		writeLearningUsecaseError(w, err)
		return
	}
	resp := map[string]any{"status": "accepted"}
	if policyID != nil {
		resp["policy_id"] = policyID.String()
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body learningdto.ProposalDecisionRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.DismissProposal(r.Context(), id, body.DecidedBy); err != nil {
		writeLearningUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	count, err := h.uc.AnalyzeAndPropose(r.Context())
	if err != nil {
		shared.WriteInternalError(w, err, "analyze failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"proposals_created": count})
}

func writeLearningUsecaseError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotPending) {
		shared.WriteError(w, http.StatusConflict, "CONFLICT", "proposal is not pending")
		return
	}
	if errors.Is(err, ErrNotFound) {
		shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "proposal not found")
		return
	}
	shared.WriteInternalError(w, err, "learning operation failed")
}
