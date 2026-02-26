// Package toolab implements the Toolab Adapter spec v1, exposing
// /_toolab/* endpoints so that the toolab CLI can discover and interact
// with Nexus during deterministic testing.
package toolab

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	toolabdto "nexus-core/internal/toolab/handler/dto"
)

// Handler serves the toolab adapter HTTP endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new toolab adapter handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts all adapter routes on the given router group.
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/manifest", h.manifest)
	rg.GET("/profile", h.profile)
	rg.GET("/schema", h.schema)
	rg.GET("/openapi", h.openapi)
	rg.GET("/suggested_flows", h.suggestedFlows)
	rg.GET("/invariants", h.invariants)
	rg.GET("/limits", h.limits)
	rg.GET("/environment", h.environment)
	rg.GET("/state/fingerprint", h.stateFingerprint)
	rg.POST("/state/snapshot", h.stateSnapshot)
	rg.POST("/state/restore", h.stateRestore)
	rg.POST("/state/reset", h.stateReset)
	rg.GET("/metrics", h.metrics)
}

func (h *Handler) manifest(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Manifest(requestBaseURL(c)))
}

func (h *Handler) profile(c *gin.Context) {
	profile, err := h.svc.Profile(c.Request.Context(), requestBaseURL(c))
	if err != nil {
		writeErr(c, http.StatusServiceUnavailable, "profile_not_available", err.Error())
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) schema(c *gin.Context) {
	value, err := h.svc.Schema(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusServiceUnavailable, "schema_not_available", err.Error())
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *Handler) openapi(c *gin.Context) {
	raw, err := h.svc.OpenAPIDocument(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusServiceUnavailable, "openapi_not_available", err.Error())
		return
	}
	c.Header("Content-Type", "application/yaml; charset=utf-8")
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", raw)
}

func (h *Handler) suggestedFlows(c *gin.Context) {
	value, err := h.svc.SuggestedFlows(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusServiceUnavailable, "suggested_flows_not_available", err.Error())
		return
	}
	c.JSON(http.StatusOK, value)
}

func (h *Handler) invariants(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Invariants())
}

func (h *Handler) limits(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Limits())
}

func (h *Handler) environment(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Environment())
}

func (h *Handler) stateFingerprint(c *gin.Context) {
	fp, err := h.svc.Fingerprint(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusOK, toolabdto.FingerprintResponse{
		Fingerprint: fp,
		Scope:       "full",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) stateSnapshot(c *gin.Context) {
	var req toolabdto.SnapshotRequest
	_ = c.ShouldBindJSON(&req)

	meta, err := h.svc.Snapshot(c.Request.Context(), req.Label)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusCreated, toolabdto.SnapshotResponse{
		SnapshotID:  meta.ID,
		Fingerprint: meta.Fingerprint,
		Label:       meta.Label,
		CreatedAt:   meta.CreatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) stateRestore(c *gin.Context) {
	var req toolabdto.RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SnapshotID == "" {
		writeErr(c, http.StatusBadRequest, "invalid_request", "snapshot_id is required")
		return
	}

	meta, err := h.svc.Restore(c.Request.Context(), req.SnapshotID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
			code = "snapshot_not_found"
		}
		writeErr(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, toolabdto.RestoreResponse{
		Restored:    true,
		SnapshotID:  meta.ID,
		Fingerprint: meta.Fingerprint,
	})
}

func (h *Handler) stateReset(c *gin.Context) {
	fp, err := h.svc.Reset(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusOK, toolabdto.ResetResponse{
		Reset:       true,
		Fingerprint: fp,
	})
}

func (h *Handler) metrics(c *gin.Context) {
	items, err := h.svc.Metrics(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	metrics := make([]toolabdto.MetricResponse, 0, len(items))
	for _, m := range items {
		metrics = append(metrics, toolabdto.MetricResponse{
			Name:   m.Name,
			Type:   m.Type,
			Value:  m.Value,
			Labels: m.Labels,
		})
	}
	c.JSON(http.StatusOK, toolabdto.MetricsResponse{
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Metrics:     metrics,
	})
}

func writeErr(c *gin.Context, status int, code, message string) {
	c.JSON(status, toolabdto.ErrorResponse{Error: code, Message: message})
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.ToLower(forwarded)
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		host = c.GetHeader("Host")
	}
	if host == "" {
		host = "localhost"
	}
	if _, _, err := net.SplitHostPort(host); err != nil && strings.Count(host, ":") == 0 {
		// Keep host as-is when no port is present.
	}
	return scheme + "://" + host
}
