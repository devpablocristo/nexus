package coreproxy

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	baseURL string
	client  *http.Client
}

func NewHandler() *Handler {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_CORE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://nexus-core:8080"
	}
	return &Handler{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (h *Handler) RegisterRoot(r *gin.Engine) {
	r.GET("/openapi.yaml", h.proxy)
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/audit", h.proxy)
	rg.GET("/audit/export", h.proxy)
	rg.GET("/approvals", h.proxy)
	rg.GET("/approvals/:id", h.proxy)
	rg.POST("/approvals/:id/approve", h.proxy)
	rg.POST("/approvals/:id/reject", h.proxy)
}

func (h *Handler) proxy(c *gin.Context) {
	target := h.baseURL + c.Request.URL.Path
	if qs := c.Request.URL.RawQuery; qs != "" {
		target += "?" + qs
	}
	var body []byte
	if c.Request.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(c.Request.Body, 2*1024*1024))
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "BAD_GATEWAY", "message": "failed to build core request"}})
		return
	}
	copyHeader := func(name string) {
		if v := c.GetHeader(name); v != "" {
			req.Header.Set(name, v)
		}
	}
	copyHeader("Authorization")
	copyHeader("Content-Type")
	copyHeader("X-NEXUS-CORE-KEY")
	copyHeader("X-NEXUS-SCOPES")
	copyHeader("X-NEXUS-ACTOR")
	copyHeader("X-NEXUS-ROLE")
	copyHeader("Idempotency-Key")
	copyHeader("X-Timeout-Ms")

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "BAD_GATEWAY", "message": "core is unavailable"}})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Status(resp.StatusCode)
	_, _ = c.Writer.Write(respBody)
}
