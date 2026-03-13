// Package billing implements Stripe-based subscription management for Nexus tenants.
package billing

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	billingdto "control-plane/internal/billing/handler/dto"
	billingdomain "control-plane/internal/billing/usecases/domain"
	"control-plane/internal/shared/authz"
	httperr "nexus/pkg/http/errors"
	"nexus/pkg/types"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/billing/status", h.getStatus)
	rg.POST("/billing/checkout", h.createCheckout)
	rg.POST("/billing/portal", h.createPortal)
	rg.GET("/billing/usage", h.getUsage)
}

func (h *Handler) getStatus(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	view, err := h.uc.GetBillingStatus(c.Request.Context(), mustOrgID(c))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toStatusDTO(view))
}

func (h *Handler) createCheckout(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req billingdto.CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	url, err := h.uc.CreateCheckoutSession(
		c.Request.Context(),
		mustOrgID(c),
		req.PlanCode,
		req.SuccessURL,
		req.CancelURL,
		actorFromCtx(c),
	)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, billingdto.CheckoutResponse{URL: url})
}

func (h *Handler) createPortal(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleWrite) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleWrite+" scope required")
		return
	}
	var req billingdto.PortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.BadRequest(c, "invalid json")
		return
	}
	url, err := h.uc.CreatePortalSession(
		c.Request.Context(),
		mustOrgID(c),
		req.ReturnURL,
		actorFromCtx(c),
	)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, billingdto.PortalResponse{URL: url})
}

func (h *Handler) getUsage(c *gin.Context) {
	if !authz.CanAccess(scopesFromCtx(c), roleFromCtx(c), authz.ScopeAdminConsoleRead) {
		httperr.Write(c, http.StatusForbidden, types.ErrCodeUnauthorized, authz.ScopeAdminConsoleRead+" scope required")
		return
	}
	summary, err := h.uc.GetUsageSummary(c.Request.Context(), mustOrgID(c))
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, toUsageDTO(summary))
}

func toStatusDTO(in billingdomain.BillingStatusView) billingdto.BillingStatusResponse {
	var currentPeriodEnd *string
	if in.CurrentPeriodEnd != nil {
		iso := in.CurrentPeriodEnd.UTC().Format(time.RFC3339)
		currentPeriodEnd = &iso
	}
	return billingdto.BillingStatusResponse{
		PlanCode:         string(in.PlanCode),
		BillingStatus:    string(in.BillingStatus),
		CurrentPeriodEnd: currentPeriodEnd,
		HardLimits: billingdto.HardLimits{
			ToolsMax:           in.HardLimits.ToolsMax,
			RunRPM:             in.HardLimits.RunRPM,
			AuditRetentionDays: in.HardLimits.AuditRetentionDays,
		},
		Usage: toUsageDTO(in.Usage),
	}
}

func toUsageDTO(in billingdomain.UsageSummary) billingdto.UsageSummaryResponse {
	return billingdto.UsageSummaryResponse{
		Period: in.Period,
		Counters: billingdto.UsageCounters{
			APICalls:        in.Counters.APICalls,
			EventsIngested:  in.Counters.EventsIngested,
			IncidentsOpened: in.Counters.IncidentsOpened,
			ActionsExecuted: in.Counters.ActionsExecuted,
		},
	}
}

func mustOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(string(types.CtxKeyOrgID))
	id, _ := v.(uuid.UUID)
	return id
}

func actorFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				return &s
			}
		}
	}
	return nil
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
		if scopes, ok := v.([]string); ok {
			return scopes
		}
	}
	return nil
}
