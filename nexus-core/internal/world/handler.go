package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus-core/internal/shared/authz"
	httperr "nexus-core/pkg/http/errors"
	ginmw "nexus-core/pkg/http/middlewares/gin"
	"nexus-core/pkg/types"
)

type Handler struct {
	svc Service
}

const (
	defaultEventsLimit      = 200
	maxEventsLimit          = 200
	streamPollInterval      = 350 * time.Millisecond
	streamKeepaliveInterval = 8 * time.Second
)

type worldEventsEnvelope struct {
	Items   []json.RawMessage `json:"items"`
	NextSeq int64             `json:"next_seq"`
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/world/runs", h.listRuns)
	rg.GET("/world/state", h.state)
	rg.GET("/world/events", h.events)
	rg.GET("/world/events/stream", h.eventsStream)
	rg.POST("/world/run/create", h.createRun)
	rg.POST("/world/replay", h.replay)
}

func (h *Handler) listRuns(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	cursor := strings.TrimSpace(c.Query("cursor"))
	out, err := h.svc.ListRuns(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), limit, cursor)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) state(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	runID := strings.TrimSpace(c.Query("run_id"))
	if runID == "" {
		httperr.BadRequest(c, "run_id is required")
		return
	}
	var stepID *int64
	if raw := strings.TrimSpace(c.Query("step_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			httperr.BadRequest(c, "step_id must be >= 0")
			return
		}
		stepID = &v
	}
	out, err := h.svc.GetState(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), runID, stepID)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) events(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	runID := strings.TrimSpace(c.Query("run_id"))
	if runID == "" {
		httperr.BadRequest(c, "run_id is required")
		return
	}
	fromSeq, err := parseFromSeq(c.Query("from_seq"))
	if err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	limit, err := parseEventsLimit(c.Query("limit"))
	if err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	out, err := h.svc.GetEvents(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), runID, fromSeq, limit)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) eventsStream(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	runID := strings.TrimSpace(c.Query("run_id"))
	if runID == "" {
		httperr.BadRequest(c, "run_id is required")
		return
	}
	fromSeq, err := parseFromSeq(c.Query("from_seq"))
	if err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	limit, err := parseEventsLimit(c.Query("limit"))
	if err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}

	if _, ok := c.Writer.(http.Flusher); !ok {
		httperr.Write(c, http.StatusInternalServerError, types.ErrCodeInternal, "streaming is not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	if err := writeSSE(c, "connected", map[string]any{
		"run_id":   runID,
		"from_seq": fromSeq,
	}); err != nil {
		return
	}

	requestID := ginmw.RequestIDFromContext(c)
	orgID := mustOrgID(c)

	streamOnce := func() error {
		out, err := h.svc.GetEvents(c.Request.Context(), orgID, requestID, runID, fromSeq, limit)
		if err != nil {
			_ = writeSSE(c, "error", map[string]any{"message": err.Error()})
			return err
		}
		envelope, err := decodeWorldEventsEnvelope(out)
		if err != nil {
			_ = writeSSE(c, "error", map[string]any{"message": "invalid world events response"})
			return err
		}
		if len(envelope.Items) > 0 {
			if err := writeSSE(c, "world.batch", envelope); err != nil {
				return err
			}
		}
		if envelope.NextSeq > fromSeq {
			fromSeq = envelope.NextSeq
			if err := writeSSE(c, "cursor", map[string]any{"next_seq": fromSeq}); err != nil {
				return err
			}
		}
		return nil
	}

	if err := streamOnce(); err != nil {
		return
	}

	pollTicker := time.NewTicker(streamPollInterval)
	defer pollTicker.Stop()
	pingTicker := time.NewTicker(streamKeepaliveInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-pollTicker.C:
			if err := streamOnce(); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := writeSSE(c, "ping", map[string]any{"next_seq": fromSeq}); err != nil {
				return
			}
		}
	}
}

func (h *Handler) createRun(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.svc.CreateRun(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), payload)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) replay(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.svc.Replay(c.Request.Context(), mustOrgID(c), ginmw.RequestIDFromContext(c), payload)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, ok := c.Get(string(types.CtxKeyOrgID))
	if !ok {
		return uuid.Nil
	}
	if id, ok := v.(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

func roleFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyRole)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func scopesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get(string(types.CtxKeyScopes)); ok {
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}

func parseFromSeq(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("from_seq must be >= 0")
	}
	return v, nil
}

func parseEventsLimit(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultEventsLimit, nil
	}
	v, err := strconv.Atoi(trimmed)
	if err != nil || v <= 0 || v > maxEventsLimit {
		return 0, fmt.Errorf("limit must be <= %d", maxEventsLimit)
	}
	return v, nil
}

func decodeWorldEventsEnvelope(in any) (worldEventsEnvelope, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return worldEventsEnvelope{}, err
	}
	var envelope worldEventsEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return worldEventsEnvelope{}, err
	}
	if envelope.Items == nil {
		envelope.Items = []json.RawMessage{}
	}
	return envelope, nil
}

func writeSSE(c *gin.Context, eventName string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\n", eventName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", raw); err != nil {
		return err
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
