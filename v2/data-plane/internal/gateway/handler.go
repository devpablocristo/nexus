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
	ListIntents(ctx context.Context, limit int) ([]gwdomain.ExecutionIntent, error)
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

func (h *Handler) writeGatewayError(w http.ResponseWriter, requestID string, err error) {
	err = toRunHTTPError(err, nil)
	var httpErr runHTTPError
	if errors.As(err, &httpErr) {
		writeError(w, httpErr.Status, requestID, httpErr.Code, httpErr.Message, httpErr.Idempotency)
	}
}

func toIntentDTO(item gwdomain.ExecutionIntent) gwdto.IntentItem {
	return gwdto.IntentItem{
		ID:         item.ID.String(),
		RequestID:  item.RequestID,
		ToolID:     item.ToolID,
		ToolName:   item.ToolName,
		PolicyID:   uuidStringPtr(item.PolicyID),
		Reason:     item.Reason,
		Status:     string(item.Status),
		ApprovalID: uuidStringPtr(item.ApprovalID),
		ExpiresAt:  item.ExpiresAt,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func uuidStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
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
