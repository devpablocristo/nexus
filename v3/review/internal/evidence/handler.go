package evidence

import (
	"context"
	"errors"
	"net/http"

	sharedhandlers "github.com/devpablocristo/core/backend/go/httpjson"
	evidencedto "github.com/devpablocristo/nexus/v3/review/internal/evidence/handler/dto"
	evidencedomain "github.com/devpablocristo/nexus/v3/review/internal/evidence/usecases/domain"
	"github.com/devpablocristo/nexus/v3/review/internal/requests"
	"github.com/devpablocristo/nexus/v3/review/internal/shared"
	"github.com/google/uuid"
)

// Port mínimo: solo lo que el handler necesita.
type evidenceUsecase interface {
	Generate(ctx context.Context, requestID uuid.UUID) (evidencedomain.EvidencePack, error)
}

// Handler expone el endpoint de evidence packs.
type Handler struct {
	uc evidenceUsecase
}

// NewHandler crea el handler de evidence.
func NewHandler(uc evidenceUsecase) *Handler {
	return &Handler{uc: uc}
}

// Register registra las rutas en el mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/requests/{id}/evidence", h.generate)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	pack, err := h.uc.Generate(r.Context(), id)
	if err != nil {
		if errors.Is(err, requests.ErrNotFound) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
			return
		}
		shared.WriteInternalError(w, err, "generate evidence pack failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toEvidenceResponse(pack))
}

// toEvidenceResponse convierte el dominio a DTO HTTP.
func toEvidenceResponse(pack evidencedomain.EvidencePack) evidencedto.EvidencePackResponse {
	resp := evidencedto.EvidencePackResponse{
		Version:     pack.Version,
		GeneratedAt: pack.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		Request: evidencedto.RequestSection{
			ID: pack.Request.ID,
			Requester: evidencedto.RequesterInfo{
				Type: pack.Request.Requester.Type,
				ID:   pack.Request.Requester.ID,
				Name: pack.Request.Requester.Name,
			},
			Action: evidencedto.ActionInfo{
				Type:           pack.Request.Action.Type,
				TargetSystem:   pack.Request.Action.TargetSystem,
				TargetResource: pack.Request.Action.TargetResource,
			},
			Params:    pack.Request.Params,
			Reason:    pack.Request.Reason,
			Context:   pack.Request.Context,
			AISummary: pack.Request.AISummary,
			CreatedAt: pack.Request.CreatedAt,
		},
		PolicyEval: evidencedto.PolicySection{
			RiskLevel:      pack.PolicyEval.RiskLevel,
			Decision:       pack.PolicyEval.Decision,
			DecisionReason: pack.PolicyEval.DecisionReason,
			PolicyID:       pack.PolicyEval.PolicyID,
			EvaluatedAt:    pack.PolicyEval.EvaluatedAt,
		},
		Signature: evidencedto.SignatureSection{
			Algorithm: pack.Signature.Algorithm,
			KeyID:     pack.Signature.KeyID,
			SignedAt:  pack.Signature.SignedAt,
			Value:     pack.Signature.Value,
		},
	}

	// Approval (opcional)
	if pack.Approval != nil {
		decisions := make([]evidencedto.DecisionEntry, 0, len(pack.Approval.Decisions))
		for _, d := range pack.Approval.Decisions {
			decisions = append(decisions, evidencedto.DecisionEntry{
				ApproverID: d.ApproverID,
				Action:     d.Action,
				Note:       d.Note,
				DecidedAt:  d.DecidedAt,
			})
		}
		resp.Approval = &evidencedto.ApprovalSection{
			ID:                pack.Approval.ID,
			Status:            pack.Approval.Status,
			BreakGlass:        pack.Approval.BreakGlass,
			RequiredApprovals: pack.Approval.RequiredApprovals,
			Decisions:         decisions,
			FinalDecidedBy:    pack.Approval.FinalDecidedBy,
			DecidedAt:         pack.Approval.DecidedAt,
		}
	}

	// Execution (opcional)
	if pack.Execution != nil {
		resp.Execution = &evidencedto.ExecutionSection{
			Status:     pack.Execution.Status,
			Result:     pack.Execution.Result,
			Error:      pack.Execution.Error,
			ExecutedAt: pack.Execution.ExecutedAt,
		}
	}

	// Attestation (opcional)
	if pack.Attestation != nil {
		resp.Attestation = &evidencedto.AttestationSection{
			ID:           pack.Attestation.ID,
			Status:       pack.Attestation.Status,
			ProviderRefs: pack.Attestation.ProviderRefs,
			Signature:    pack.Attestation.Signature,
			Attester:     pack.Attestation.Attester,
			Metadata:     pack.Attestation.Metadata,
			CreatedAt:    pack.Attestation.CreatedAt,
		}
	}

	// Timeline
	resp.Timeline = make([]evidencedto.TimelineEntry, 0, len(pack.Timeline))
	for _, e := range pack.Timeline {
		resp.Timeline = append(resp.Timeline, evidencedto.TimelineEntry{
			Event:   e.Event,
			Actor:   e.Actor,
			At:      e.At,
			Summary: e.Summary,
			Data:    e.Data,
		})
	}

	return resp
}
