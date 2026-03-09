// Package operatorproxy exposes internal-only bridge routes for operator services.
package operatorproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nexus-core/internal/shared/leasehttp"
)

const headerOperatorKey = "X-NEXUS-AI-KEY"

type Handler struct {
	saasURL         string
	saasAPIKey      string
	saasInternalKey string
	internalKey     string
	client          *http.Client
}

type appendEventRequest struct {
	OrgID     string         `json:"org_id" binding:"required"`
	EventType string         `json:"event_type" binding:"required"`
	Payload   map[string]any `json:"payload"`
}

func NewHandlerFromEnv() *Handler {
	timeoutMS := 1000
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	saasAPIKey := strings.TrimSpace(os.Getenv("NEXUS_OPERATOR_API_KEY"))
	if saasAPIKey == "" {
		saasAPIKey = strings.TrimSpace(os.Getenv("NEXUS_SAAS_API_KEY"))
	}
	return &Handler{
		saasURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/"),
		saasAPIKey:      saasAPIKey,
		saasInternalKey: strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY")),
		internalKey:     strings.TrimSpace(os.Getenv("NEXUS_AI_OPERATORS_INTERNAL_KEY")),
		client:          &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
	}
}

func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/internal/operators")
	g.Use(h.requireInternalKey())
	g.Use(leasehttp.NewFromEnv().Middleware())
	g.GET("/events", h.forward(http.MethodGet, "/v1/events", "audit:read,admin:console:read", "operator/observer"))
	g.POST("/events/append", h.appendEvent())
	g.POST("/actions/apply", h.forward(http.MethodPost, "/v1/actions/apply", "admin:console:write", "operator/responder"))
	g.POST("/incidents", h.forward(http.MethodPost, "/v1/incidents", "admin:console:write", "operator/responder"))
	g.POST("/policy-proposals", h.forward(http.MethodPost, "/v1/policy-proposals", "admin:console:write", "operator/policy-proposer"))
}

func (h *Handler) appendEvent() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.saasURL == "" || h.saasInternalKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "saas proxy not configured"})
			return
		}
		var reqBody appendEventRequest
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		reqBody.OrgID = strings.TrimSpace(reqBody.OrgID)
		reqBody.EventType = strings.TrimSpace(reqBody.EventType)
		if reqBody.OrgID == "" || reqBody.EventType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "org_id and event_type are required"})
			return
		}
		if reqBody.Payload == nil {
			reqBody.Payload = map[string]any{}
		}
		raw, err := json.Marshal(reqBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		upReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.saasURL+"/internal/events", bytes.NewReader(raw))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build proxy request"})
			return
		}
		upReq.Header.Set("Content-Type", "application/json")
		upReq.Header.Set("X-NEXUS-SAAS-KEY", h.saasInternalKey)
		copyExecutionLeaseHeaders(c.Request, upReq)
		resp, err := h.client.Do(upReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream unavailable"})
			return
		}
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		c.Status(resp.StatusCode)
		if len(payload) > 0 {
			c.Header("Content-Type", "application/json")
			_, _ = c.Writer.Write(payload)
		}
	}
}

func (h *Handler) requireInternalKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.internalKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "operator internal key not configured"})
			c.Abort()
			return
		}
		got := strings.TrimSpace(c.GetHeader(headerOperatorKey))
		if got == "" || got != h.internalKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) forward(method, targetPath, scopes, actor string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.saasURL == "" || h.saasAPIKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "saas proxy not configured"})
			return
		}
		targetURL := h.saasURL + targetPath
		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			targetURL += "?" + rawQuery
		}

		var bodyReader io.Reader
		if c.Request.Body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
			raw, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			bodyReader = bytes.NewReader(raw)
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL, bodyReader)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build proxy request"})
			return
		}
		req.Header.Set("X-NEXUS-CORE-KEY", h.saasAPIKey)
		req.Header.Set("X-NEXUS-SCOPES", scopes)
		req.Header.Set("X-NEXUS-ACTOR", actor)
		req.Header.Set("Content-Type", "application/json")
		copyExecutionLeaseHeaders(c.Request, req)

		resp, err := h.client.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream unavailable"})
			return
		}
		defer resp.Body.Close()

		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read upstream response"})
			return
		}
		c.Status(resp.StatusCode)
		if len(payload) > 0 {
			c.Header("Content-Type", "application/json")
			_, _ = c.Writer.Write(payload)
		}
	}
}

func copyExecutionLeaseHeaders(src *http.Request, dst *http.Request) {
	for _, header := range []string{
		"Authorization",
		"X-Nexus-Execution-Token",
		"X-Nexus-Lease-Id",
		"X-Nexus-Intent-Id",
		"X-Nexus-Credential-Mode",
		"X-Nexus-Tool-Name",
		"X-Nexus-Risk-Class",
		"X-Nexus-Credential-Scope",
		"X-Nexus-Credential-Provider",
		"X-Nexus-Target-Env",
	} {
		if value := strings.TrimSpace(src.Header.Get(header)); value != "" {
			dst.Header.Set(header, value)
		}
	}
}
