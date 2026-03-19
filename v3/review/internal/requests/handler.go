package requests

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/handlers"
	requestdomain "github.com/devpablocristo/nexus/v3/review/internal/requests/usecases/domain"
	requestdto "github.com/devpablocristo/nexus/v3/review/internal/requests/handler/dto"
	"github.com/devpablocristo/nexus/v3/review/internal/shared"
)

// Port mínimo: solo lo que el handler necesita
type requestUsecase interface {
	Submit(ctx context.Context, in SubmitInput) (SubmitOutput, error)
	Simulate(ctx context.Context, in SubmitInput) (SimulateOutput, error)
	GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error)
	List(ctx context.Context, status, actionType string, limit int) ([]requestdomain.Request, error)
	ReportResult(ctx context.Context, requestID uuid.UUID, in ReportResultInput) error
}

type Handler struct {
	uc requestUsecase
}

func NewHandler(uc requestUsecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/requests", h.submit)
	mux.HandleFunc("POST /v1/requests/simulate", h.simulate)
	mux.HandleFunc("GET /v1/requests", h.list)
	mux.HandleFunc("GET /v1/requests/{id}", h.getByID)
	mux.HandleFunc("POST /v1/requests/{id}/result", h.reportResult)
}

func (h *Handler) simulate(w http.ResponseWriter, r *http.Request) {
	var body requestdto.SimulateRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.ActionType == "" {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "action_type is required")
		return
	}

	out, err := h.uc.Simulate(r.Context(), SubmitInput{
		RequesterType:  body.RequesterType,
		RequesterID:    body.RequesterID,
		RequesterName:  body.RequesterName,
		ActionType:     body.ActionType,
		TargetSystem:   body.TargetSystem,
		TargetResource: body.TargetResource,
		Params:         body.Params,
		Reason:         body.Reason,
		Context:        body.Context,
	})
	if err != nil {
		slog.Error("simulate failed", "error", err)
		shared.WriteInternalError(w, err, "simulate request")
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, requestdto.SimulateResponse{
		Decision:             out.Decision,
		RiskLevel:            out.RiskLevel,
		DecisionReason:       out.DecisionReason,
		Status:               out.Status,
		PolicyMatched:        out.PolicyMatched,
		RiskAssessment:       out.RiskAssessment,
		WouldRequireApproval: out.WouldRequireApproval,
		AISummary:            out.AISummary,
	})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	var body requestdto.SubmitRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.RequesterType == "" || body.RequesterID == "" || body.ActionType == "" {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "requester_type, requester_id and action_type are required")
		return
	}
	var idemKey *string
	if k := r.Header.Get("Idempotency-Key"); k != "" {
		if len(k) > shared.MaxIdempotencyKeyLen {
			shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "idempotency key too long")
			return
		}
		idemKey = &k
	}
	out, err := h.uc.Submit(r.Context(), SubmitInput{
		IdempotencyKey: idemKey,
		RequesterType:  body.RequesterType,
		RequesterID:    body.RequesterID,
		RequesterName:  body.RequesterName,
		ActionType:     body.ActionType,
		TargetSystem:   body.TargetSystem,
		TargetResource: body.TargetResource,
		Params:         body.Params,
		Reason:         body.Reason,
		Context:        body.Context,
	})
	if err != nil {
		shared.WriteInternalError(w, err, "request submission failed")
		return
	}
	resp := requestdto.SubmitResponse{
		RequestID:      out.RequestID.String(),
		Decision:       out.Decision,
		RiskLevel:      out.RiskLevel,
		DecisionReason: out.DecisionReason,
		Status:         out.Status,
		AISummary:      out.AISummary,
		AIDegraded:     out.AIDegraded,
	}
	if out.Approval != nil {
		resp.Approval = &requestdto.ApprovalPayload{
			ID:        out.Approval.ID.String(),
			ExpiresAt: out.Approval.ExpiresAt.Format(time.RFC3339),
		}
	}
	sharedhandlers.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	actionType := q.Get("action_type")
	limit := shared.DefaultListLimit
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= shared.MaxListLimit {
			limit = parsed
		}
	}
	list, err := h.uc.List(r.Context(), status, actionType, limit)
	if err != nil {
		shared.WriteInternalError(w, err, "list requests failed")
		return
	}
	out := make([]requestdto.RequestResponse, 0, len(list))
	for _, req := range list {
		out = append(out, toRequestResponse(req))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	req, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
			return
		}
		shared.WriteInternalError(w, err, "get request failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toRequestResponse(req))
}

func (h *Handler) reportResult(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body requestdto.ReportResultRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	err = h.uc.ReportResult(r.Context(), id, ReportResultInput{
		Success:      body.Success,
		Result:       body.Result,
		DurationMs:   body.DurationMs,
		ErrorMessage: body.ErrorMessage,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			shared.WriteError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
			return
		}
		shared.WriteInternalError(w, err, "report result failed")
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Helpers ---

func toRequestResponse(req requestdomain.Request) requestdto.RequestResponse {
	return requestdto.RequestResponse{
		ID:             req.ID.String(),
		RequesterType:  string(req.RequesterType),
		RequesterID:    req.RequesterID,
		RequesterName:  req.RequesterName,
		ActionType:     req.ActionType,
		TargetSystem:   req.TargetSystem,
		TargetResource: req.TargetResource,
		Params:         req.Params,
		Reason:         req.Reason,
		RiskLevel:      string(req.RiskLevel),
		Decision:       string(req.Decision),
		DecisionReason: req.DecisionReason,
		Status:         string(req.Status),
		AISummary:      req.AISummary,
		AIDegraded:     req.AIDegraded,
		CreatedAt:      req.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      req.UpdatedAt.Format(time.RFC3339),
	}
}


// logAuditError loguea errores de audit sin fallar la request (best-effort).
func logAuditError(err error, requestID uuid.UUID, event string) {
	if err != nil {
		slog.Error("audit event failed", "error", err, "request_id", requestID, "event", event)
	}
}
