package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	auditdto "nexus-gateway/internal/audit/handler/dto"
	audituc "nexus-gateway/internal/audit/usecases"
	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	ginmw "nexus-gateway/pkg/http/middlewares/gin"
	"nexus-gateway/pkg/types"
)

type Handler struct {
	svc audituc.Service
}

func NewHandler(svc audituc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/audit", h.query)
}

func (h *Handler) query(c *gin.Context) {
	orgID := mustOrgID(c)
	q, err := parseQuery(c)
	if err != nil {
		writeUCError(c, err)
		return
	}
	items, err := h.svc.Query(c.Request.Context(), orgID, q)
	if err != nil {
		writeUCError(c, err)
		return
	}
	out := auditdto.ListAuditResponse{Items: make([]auditdto.AuditItem, 0, len(items))}
	for _, ev := range items {
		out.Items = append(out.Items, toItem(ev))
	}
	c.JSON(http.StatusOK, out)
}

func parseQuery(c *gin.Context) (auditdomain.Query, error) {
	var q auditdomain.Query
	if v := c.Query("tool_name"); v != "" {
		q.ToolName = &v
	}
	if v := c.Query("decision"); v != "" {
		d := auditdomain.Decision(v)
		q.Decision = &d
	}
	if v := c.Query("status"); v != "" {
		s := auditdomain.Status(v)
		q.Status = &s
	}
	if v := c.Query("from"); v != "" {
		tm, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return auditdomain.Query{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid from")
		}
		q.From = &tm
	}
	if v := c.Query("to"); v != "" {
		tm, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return auditdomain.Query{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid to")
		}
		q.To = &tm
	}
	if v := c.Query("limit"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			return auditdomain.Query{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid limit")
		}
		q.Limit = i
	}
	return q, nil
}

func toItem(ev auditdomain.AuditEvent) auditdto.AuditItem {
	var errObj *auditdto.ErrorObj
	if ev.ErrorCode != nil || ev.ErrorMessage != nil {
		var code, msg string
		if ev.ErrorCode != nil {
			code = *ev.ErrorCode
		}
		if ev.ErrorMessage != nil {
			msg = *ev.ErrorMessage
		}
		errObj = &auditdto.ErrorObj{Code: code, Message: msg}
	}
	return auditdto.AuditItem{
		RequestID: ev.RequestID,
		ToolName:  ev.ToolName,
		Actor:     ev.Actor,
		Decision:  string(ev.Decision),
		Status:    string(ev.Status),
		Reason:    ev.Reason,
		LatencyMS: ev.LatencyMS,
		CreatedAt: ev.CreatedAt,
		Input:     ev.InputRedacted,
		Context:   ev.ContextRedacted,
		Output:    ev.OutputRedacted,
		Error:     errObj,
	}
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

func writeUCError(c *gin.Context, err error) {
	var he types.HTTPError
	if errors.As(err, &he) {
		writeError(c, he.Status, he.Code, he.Message)
		return
	}
	writeError(c, http.StatusInternalServerError, types.ErrCodeInternal, "internal error")
}

func writeError(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, types.ErrorResponse{
		RequestID: ginmw.RequestIDFromContext(c),
		Error:     types.APIError{Code: code, Message: msg},
	})
}
