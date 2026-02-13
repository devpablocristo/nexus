package audit

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	auditdto "nexus-gateway/internal/audit/handler/dto"
	auditdomain "nexus-gateway/internal/audit/usecases/domain"
	httperr "nexus-gateway/pkg/http/errors"
	"nexus-gateway/pkg/types"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/audit", h.query)
	rg.GET("/audit/export", h.export)
}

func (h *Handler) query(c *gin.Context) {
	orgID := mustOrgID(c)
	q, err := parseQuery(c)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	items, err := h.svc.Query(c.Request.Context(), orgID, q)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	out := auditdto.ListAuditResponse{Items: make([]auditdto.AuditItem, 0, len(items))}
	for _, ev := range items {
		out.Items = append(out.Items, toItem(ev))
	}
	c.JSON(200, out)
}

func parseQuery(c *gin.Context) (auditdomain.Query, error) {
	var q auditdomain.Query
	if v := c.Query("tool_name"); v != "" {
		q.ToolName = &v
	}
	if v := c.Query("decision"); v != "" {
		d := auditdomain.Decision(v)
		if d != auditdomain.DecisionAllow && d != auditdomain.DecisionDeny {
			return auditdomain.Query{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid decision")
		}
		q.Decision = &d
	}
	if v := c.Query("status"); v != "" {
		s := auditdomain.Status(v)
		if s != auditdomain.StatusSuccess && s != auditdomain.StatusError && s != auditdomain.StatusBlocked {
			return auditdomain.Query{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid status")
		}
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
		RequestID:          ev.RequestID,
		OrgID:              ev.OrgID.String(),
		ToolName:           ev.ToolName,
		Actor:              ev.Actor,
		Role:               ev.ActorRole,
		Scopes:             ev.ActorScopes,
		Decision:           string(ev.Decision),
		Status:             string(ev.Status),
		Reason:             ev.Reason,
		LatencyMS:          ev.LatencyMS,
		IdempotencyPresent: ev.IdempotencyPresent,
		IdempotencyOutcome: ev.IdempotencyOutcome,
		TimeoutMS:          ev.TimeoutMS,
		BudgetRemainingMS:  ev.BudgetRemainingMSAtExecute,
		StageDurationsMS:   ev.StageDurationsMS,
		PrevEventHash:      ev.PrevEventHash,
		EventHash:          ev.EventHash,
		HashAlgo:           "sha256",
		CreatedAt:          ev.CreatedAt,
		Input:              ev.InputRedacted,
		Context:            ev.ContextRedacted,
		DLPSummary:         ev.DLPSummary,
		Output:             ev.OutputRedacted,
		Error:              errObj,
	}
}

func (h *Handler) export(c *gin.Context) {
	orgID := mustOrgID(c)
	q, err := parseQuery(c)
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	if q.From != nil && q.To != nil && q.From.After(*q.To) {
		httperr.Write(c, http.StatusBadRequest, types.ErrCodeExportRangeInvalid, "from must be <= to")
		return
	}
	q.OrderAsc = true
	format := c.DefaultQuery("format", "jsonl")
	switch format {
	case "jsonl":
		h.exportJSONL(c, orgID, q)
	case "csv":
		h.exportCSV(c, orgID, q)
	default:
		httperr.Write(c, http.StatusBadRequest, types.ErrCodeExportFormatInvalid, "format must be jsonl|csv")
	}
}

func (h *Handler) exportJSONL(c *gin.Context, orgID uuid.UUID, q auditdomain.Query) {
	c.Header("Content-Type", "application/x-ndjson")
	c.Status(http.StatusOK)
	enc := json.NewEncoder(c.Writer)
	err := h.svc.StreamByFilters(c.Request.Context(), orgID, q, 200, func(ev auditdomain.AuditEvent) error {
		return enc.Encode(toItem(ev))
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
}

func (h *Handler) exportCSV(c *gin.Context, orgID uuid.UUID, q auditdomain.Query) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{
		"created_at", "org_id", "request_id", "tool_name", "actor", "role", "scopes",
		"decision", "status", "latency_ms", "error_code", "idempotency_present", "idempotency_outcome",
		"timeout_ms", "budget_remaining_ms_at_execute", "stage_durations_ms", "dlp_summary",
		"input", "context", "output", "prev_event_hash", "event_hash", "hash_algo",
	})
	err := h.svc.StreamByFilters(c.Request.Context(), orgID, q, 200, func(ev auditdomain.AuditEvent) error {
		item := toItem(ev)
		scopesJSON, _ := json.Marshal(item.Scopes)
		stageJSON, _ := json.Marshal(item.StageDurationsMS)
		dlpJSON, _ := json.Marshal(item.DLPSummary)
		inJSON, _ := json.Marshal(item.Input)
		ctxJSON, _ := json.Marshal(item.Context)
		outJSON, _ := json.Marshal(item.Output)
		var errCode string
		if item.Error != nil {
			errCode = item.Error.Code
		}
		return writer.Write([]string{
			item.CreatedAt.Format(time.RFC3339),
			item.OrgID,
			item.RequestID,
			item.ToolName,
			derefPtr(item.Actor),
			derefPtr(item.Role),
			string(scopesJSON),
			item.Decision,
			item.Status,
			strconv.Itoa(item.LatencyMS),
			errCode,
			strconv.FormatBool(item.IdempotencyPresent),
			item.IdempotencyOutcome,
			intPtr(item.TimeoutMS),
			intPtr(item.BudgetRemainingMS),
			string(stageJSON),
			string(dlpJSON),
			string(inJSON),
			string(ctxJSON),
			string(outJSON),
			derefPtr(item.PrevEventHash),
			derefPtr(item.EventHash),
			"sha256",
		})
	})
	if err != nil {
		httperr.WriteFrom(c, err)
		return
	}
	writer.Flush()
}

func intPtr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func derefPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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

// centralized error handling via pkg/http/errors
