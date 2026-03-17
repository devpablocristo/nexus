package requests

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	requestdomain "github.com/devpablocristo/nexus/review-v1/internal/requests/usecases/domain"
	requestdto "github.com/devpablocristo/nexus/review-v1/internal/requests/handler/dto"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/requests", h.submit)
	mux.HandleFunc("GET /v1/requests", h.list)
	mux.HandleFunc("GET /v1/requests/{id}", h.getByID)
	mux.HandleFunc("POST /v1/requests/{id}/result", h.reportResult)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	var body requestdto.SubmitRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		writeRequestError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
		return
	}
	if body.RequesterType == "" || body.RequesterID == "" || body.ActionType == "" {
		writeRequestError(w, http.StatusBadRequest, "VALIDATION", "requester_type, requester_id and action_type are required")
		return
	}
	var idemKey *string
	if k := r.Header.Get("Idempotency-Key"); k != "" {
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
		writeRequestError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
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
	limit := 50
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	list, err := h.uc.List(r.Context(), status, actionType, limit)
	if err != nil {
		writeRequestError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
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
		writeRequestError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	req, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeRequestError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
			return
		}
		writeRequestError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toRequestResponse(req))
}

func (h *Handler) reportResult(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeRequestError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return
	}
	var body requestdto.ReportResultRequest
	if err := sharedhandlers.DecodeJSON(r, &body); err != nil {
		writeRequestError(w, http.StatusBadRequest, "VALIDATION", "invalid json")
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
			writeRequestError(w, http.StatusNotFound, "NOT_FOUND", "request not found")
			return
		}
		writeRequestError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
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

type requestErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeRequestError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, requestErrorResponse{Code: code, Message: message})
}

// logAuditError loguea errores de audit sin fallar la request (best-effort).
func logAuditError(err error, requestID uuid.UUID, event string) {
	if err != nil {
		slog.Error("audit event failed", "error", err, "request_id", requestID, "event", event)
	}
}
