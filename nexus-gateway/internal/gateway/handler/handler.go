package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gwdto "nexus-gateway/internal/gateway/handler/dto"
	gwuc "nexus-gateway/internal/gateway/usecases"
	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
	ginmw "nexus-gateway/pkg/http/middlewares/gin"
	"nexus-gateway/pkg/types"
)

type Handler struct {
	svc gwuc.Service
}

func NewHandler(svc gwuc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/run", h.run)
}

func (h *Handler) run(c *gin.Context) {
	var req gwdto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			RequestID: ginmw.RequestIDFromContext(c),
			Error:     types.APIError{Code: types.ErrCodeValidation, Message: "invalid json"},
		})
		return
	}

	orgID := mustOrgID(c)
	actor := actorFromCtx(c)
	rid := req.RequestID
	if rid == "" {
		rid = uuid.NewString()
	}

	resp, err := h.svc.Run(c.Request.Context(), orgID, actor, gwdomain.RunRequest{
		RequestID: rid,
		ToolName:  req.ToolName,
		Input:     req.Input,
		Context:   req.Context,
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			RequestID: rid,
			Error:     types.APIError{Code: types.ErrCodeInternal, Message: "internal error"},
		})
		return
	}

	if resp.Status == gwdomain.RunStatusSuccess {
		c.JSON(resp.HTTPStatus, gwdto.RunSuccessResponse{
			RequestID: resp.RequestID,
			Decision:  string(resp.Decision),
			ToolName:  resp.ToolName,
			Status:    string(resp.Status),
			Result:    resp.Result,
			LatencyMS: resp.LatencyMS,
		})
		return
	}

	if resp.Status == gwdomain.RunStatusBlocked {
		c.JSON(resp.HTTPStatus, gwdto.RunBlockedResponse{
			RequestID: resp.RequestID,
			Decision:  string(resp.Decision),
			Status:    string(resp.Status),
			Reason:    deref(resp.Reason),
			Error:     types.APIError{Code: deref(resp.ErrorCode), Message: deref(resp.ErrorMsg)},
			LatencyMS: resp.LatencyMS,
		})
		return
	}

	c.JSON(resp.HTTPStatus, gwdto.RunErrorResponse{
		RequestID: resp.RequestID,
		Decision:  string(resp.Decision),
		Status:    string(resp.Status),
		Error:     types.APIError{Code: deref(resp.ErrorCode), Message: deref(resp.ErrorMsg)},
		LatencyMS: resp.LatencyMS,
	})
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

func actorFromCtx(c *gin.Context) *string {
	if v, ok := c.Get(string(types.CtxKeyActor)); ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
