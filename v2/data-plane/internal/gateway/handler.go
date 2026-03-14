package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	gwdto "nexus/v2/data-plane/internal/gateway/handler/dto"
	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

type runUsecase interface {
	Run(ctx context.Context, req gwdomain.RunRequest) (gwdomain.RunResponse, error)
	GetIntent(ctx context.Context, intentID uuid.UUID) (gwdomain.ExecutionIntent, error)
	GetIntentPreflight(ctx context.Context, intentID uuid.UUID) (gwdomain.PreflightReview, error)
	IssueExecutionLease(ctx context.Context, intentID uuid.UUID) (gwdomain.ExecutionLease, error)
	ListIntents(ctx context.Context, limit int) ([]gwdomain.ExecutionIntent, error)
	ExecuteIntentWithLease(ctx context.Context, intentID, leaseID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error)
}

type Handler struct {
	run runUsecase
}

func NewHandler(runUC runUsecase) *Handler {
	return &Handler{run: runUC}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/run", h.runTool)
	mux.HandleFunc("GET /v1/run/intents", h.listIntents)
	mux.HandleFunc("GET /v1/run/intents/{id}", h.getIntent)
	mux.HandleFunc("GET /v1/run/intents/{id}/preflight", h.getIntentPreflight)
	mux.HandleFunc("POST /v1/run/intents/{id}/lease", h.issueExecutionLease)
	mux.HandleFunc("POST /v1/run/intents/{id}/execute", h.executeIntent)
}

func (h *Handler) runTool(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	var req gwdto.RunRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, reqID, "INVALID_JSON", "invalid json", nil)
		return
	}
	if req.RequestID != "" {
		reqID = req.RequestID
	}
	if req.ToolName == "" && req.ToolID == "" {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "tool_name or tool_id required", nil)
		return
	}
	if req.Input == nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "input required", nil)
		return
	}

	resp, err := h.run.Run(r.Context(), gwdomain.RunRequest{
		RequestID:      reqID,
		ToolName:       req.ToolName,
		ToolID:         req.ToolID,
		IdempotencyKey: parseIdempotencyKey(r.Header.Get("Idempotency-Key")),
		TimeoutMS:      req.TimeoutMS,
		Input:          req.Input,
		Context:        req.Context,
	})
	if err != nil {
		err = toRunHTTPError(err, nil)
		var httpErr runHTTPError
		if errors.As(err, &httpErr) {
			writeError(w, httpErr.Status, reqID, httpErr.Code, httpErr.Message, httpErr.Idempotency)
			return
		}
		return
	}

	writeRunResponse(w, resp)
}

func writeError(w http.ResponseWriter, status int, requestID, code, message string, idem *gwdomain.IdempotencyMeta) {
	if idem != nil {
		writeIdempotencyHeader(w, idem.Outcome)
	}
	writeJSON(w, status, gwdto.ErrorResponse{
		RequestID: requestID,
		Error: gwdto.ErrorObject{
			Code:    code,
			Message: message,
		},
		Idempotency: toIdempotencyDTOFromPtr(idem),
	})
}

func (h *Handler) listIntents(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "limit must be a positive integer", nil)
			return
		}
		limit = parsed
	}

	items, err := h.run.ListIntents(r.Context(), limit)
	if err != nil {
		h.writeGatewayError(w, reqID, err)
		return
	}

	resp := gwdto.ListIntentsResponse{Items: make([]gwdto.IntentItem, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toIntentDTO(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getIntent(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "invalid id", nil)
		return
	}

	item, err := h.run.GetIntent(r.Context(), intentID)
	if err != nil {
		h.writeGatewayError(w, reqID, err)
		return
	}
	writeJSON(w, http.StatusOK, toIntentDTO(item))
}

func (h *Handler) getIntentPreflight(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "invalid id", nil)
		return
	}

	item, err := h.run.GetIntentPreflight(r.Context(), intentID)
	if err != nil {
		h.writeGatewayError(w, reqID, err)
		return
	}
	writeJSON(w, http.StatusOK, toPreflightReviewDTO(item))
}

func (h *Handler) executeIntent(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "invalid id", nil)
		return
	}

	var req gwdto.ExecuteIntentRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.LeaseID == "" {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "lease_id required", nil)
		return
	}

	leaseID, err := uuid.Parse(req.LeaseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "invalid lease_id", nil)
		return
	}

	resp, err := h.run.ExecuteIntentWithLease(r.Context(), intentID, leaseID, parseTimeoutMS(r.Header.Get("X-Timeout-Ms")))
	if err != nil {
		h.writeGatewayError(w, reqID, err)
		return
	}
	writeRunResponse(w, resp)
}

func (h *Handler) issueExecutionLease(w http.ResponseWriter, r *http.Request) {
	reqID := newRequestID()

	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, reqID, "VALIDATION", "invalid id", nil)
		return
	}

	lease, err := h.run.IssueExecutionLease(r.Context(), intentID)
	if err != nil {
		h.writeGatewayError(w, reqID, err)
		return
	}
	writeJSON(w, http.StatusCreated, toExecutionLeaseDTO(lease))
}

func (h *Handler) writeGatewayError(w http.ResponseWriter, requestID string, err error) {
	err = toRunHTTPError(err, nil)
	var httpErr runHTTPError
	if errors.As(err, &httpErr) {
		writeError(w, httpErr.Status, requestID, httpErr.Code, httpErr.Message, httpErr.Idempotency)
	}
}

func toIntentDTO(item gwdomain.ExecutionIntent) gwdto.IntentItem {
	return gwdto.IntentItem{
		ID:                   item.ID.String(),
		RequestID:            item.RequestID,
		ToolID:               item.ToolID,
		ToolName:             item.ToolName,
		PolicyID:             uuidStringPtr(item.PolicyID),
		RiskClass:            string(item.RiskClass),
		Reason:               item.Reason,
		Status:               string(item.Status),
		PreflightStatus:      string(item.PreflightStatus),
		PreflightSummary:     cloneMap(item.PreflightSummary),
		PreflightCompletedAt: item.PreflightCompletedAt,
		ApprovalID:           uuidStringPtr(item.ApprovalID),
		ExpiresAt:            item.ExpiresAt,
		ExecutedAt:           item.ExecutedAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func toPreflightReviewDTO(item gwdomain.PreflightReview) gwdto.PreflightReviewResponse {
	return gwdto.PreflightReviewResponse{
		IntentID:     item.IntentID.String(),
		ToolName:     item.ToolName,
		RiskClass:    string(item.RiskClass),
		Reason:       item.Reason,
		Status:       string(item.Status),
		Summary:      cloneMap(item.Summary),
		CompletedAt:  item.CompletedAt,
		ApprovalID:   uuidStringPtr(item.ApprovalID),
		IntentStatus: string(item.IntentStatus),
	}
}

func toExecutionLeaseDTO(lease gwdomain.ExecutionLease) gwdto.ExecutionLeaseItem {
	return gwdto.ExecutionLeaseItem{
		ID:              lease.ID.String(),
		IntentID:        lease.IntentID.String(),
		ToolName:        lease.ToolName,
		RiskClass:       string(lease.RiskClass),
		Status:          string(lease.Status),
		CredentialMode:  lease.CredentialMode,
		CredentialHints: cloneMap(lease.CredentialHints),
		ExpiresAt:       lease.ExpiresAt,
		UsedAt:          lease.UsedAt,
		CreatedAt:       lease.CreatedAt,
	}
}

func uuidStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func uuidStringValue(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func parseTimeoutMS(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func writeRunResponse(w http.ResponseWriter, resp gwdomain.RunResponse) {
	writeIdempotencyHeader(w, resp.Idempotency.Outcome)
	status := resp.HTTPStatus
	if status == 0 {
		status = http.StatusOK
	}

	writeJSON(w, status, gwdto.RunResponse{
		RequestID:   resp.RequestID,
		Decision:    resp.Decision,
		ToolName:    resp.ToolName,
		Status:      resp.Status,
		Reason:      resp.Reason,
		Result:      resp.Result,
		LatencyMS:   resp.LatencyMS,
		IntentID:    resp.IntentID,
		ApprovalID:  resp.ApprovalID,
		Idempotency: toIdempotencyDTO(resp.Idempotency),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func toIdempotencyDTO(meta gwdomain.IdempotencyMeta) *gwdto.IdempotencyDTO {
	if !meta.Present && meta.Outcome == "" {
		return nil
	}
	return &gwdto.IdempotencyDTO{
		Present: meta.Present,
		Outcome: string(meta.Outcome),
	}
}

func toIdempotencyDTOFromPtr(meta *gwdomain.IdempotencyMeta) *gwdto.IdempotencyDTO {
	if meta == nil {
		return nil
	}
	return toIdempotencyDTO(*meta)
}
