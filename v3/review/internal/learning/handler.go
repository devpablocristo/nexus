package learning

import (
	"context"
	"net/http"

	"github.com/devpablocristo/core/backend/go/httpjson"
	learningdto "github.com/devpablocristo/nexus/v3/review/internal/learning/handler/dto"
	learningdomain "github.com/devpablocristo/nexus/v3/review/internal/learning/usecases/domain"
	"github.com/google/uuid"
	"github.com/devpablocristo/core/backend/go/domainerr"
)

const defaultListLimit = 50

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
	list, err := h.uc.ListPendingProposals(r.Context(), defaultListLimit)
	if err != nil {
		httpjson.WriteFlatInternalError(w, err, "list proposals failed")
		return
	}
	out := make([]learningdto.ProposalResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toProposalResponse(p))
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	p, err := h.uc.GetProposalByID(r.Context(), id)
	if err != nil {
		writeLearningUsecaseError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, toProposalResponse(p))
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body learningdto.ProposalDecisionRequest
	if err := httpjson.DecodeJSON(r, &body); err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
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
	httpjson.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body learningdto.ProposalDecisionRequest
	if err := httpjson.DecodeJSON(r, &body); err != nil {
		httpjson.WriteFlatError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if err := h.uc.DismissProposal(r.Context(), id, body.DecidedBy); err != nil {
		writeLearningUsecaseError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	count, err := h.uc.AnalyzeAndPropose(r.Context())
	if err != nil {
		httpjson.WriteFlatInternalError(w, err, "analyze failed")
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"proposals_created": count})
}

// toProposalResponse convierte entidad de dominio a DTO HTTP.
func toProposalResponse(p learningdomain.PolicyProposal) learningdto.ProposalResponse {
	resp := learningdto.ProposalResponse{
		ID:                  p.ID.String(),
		ProposedName:        p.ProposedName,
		ProposedDescription: p.ProposedDescription,
		ProposedExpression:  p.ProposedExpression,
		ProposedEffect:      p.ProposedEffect,
		ProposedActionType:  p.ProposedActionType,
		ProposedPriority:    p.ProposedPriority,
		PatternSummary:      p.PatternSummary,
		Confidence:          p.Confidence,
		SampleSize:          p.SampleSize,
		TimeWindow:          p.TimeWindow,
		Status:              string(p.Status),
		DecidedBy:           p.DecidedBy,
		CreatedAt:           p.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if p.DecidedAt != nil {
		s := p.DecidedAt.Format("2006-01-02T15:04:05Z")
		resp.DecidedAt = &s
	}
	if p.PolicyID != nil {
		s := p.PolicyID.String()
		resp.PolicyID = &s
	}
	return resp
}

func writeLearningUsecaseError(w http.ResponseWriter, err error) {
	if domainerr.IsConflict(err) {
		httpjson.WriteFlatError(w, http.StatusConflict, "CONFLICT", "proposal is not pending")
		return
	}
	if domainerr.IsNotFound(err) {
		httpjson.WriteFlatError(w, http.StatusNotFound, "NOT_FOUND", "proposal not found")
		return
	}
	httpjson.WriteFlatInternalError(w, err, "learning operation failed")
}
