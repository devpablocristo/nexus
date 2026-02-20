package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-core/internal/audit/repository/models"
	auditdomain "nexus-core/internal/audit/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, ev auditdomain.AuditEvent) error {
	inB, _ := json.Marshal(ev.InputRedacted)
	ctxB, _ := json.Marshal(ev.ContextRedacted)
	dlpB, _ := json.Marshal(ev.DLPSummary)
	outB, _ := json.Marshal(ev.OutputRedacted)
	scopesB, _ := json.Marshal(ev.ActorScopes)
	stageDurB, _ := json.Marshal(ev.StageDurationsMS)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prevHash *string
		var last models.AuditEvent
		err := tx.Select("event_hash").
			Where("org_id = ?", ev.OrgID).
			Where("event_hash is not null").
			Order("created_at desc, id desc").
			Limit(1).
			Take(&last).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && last.EventHash != nil && *last.EventHash != "" {
			prevHash = last.EventHash
		}
		eventHash, err := computeEventHash(prevHash, ev, inB, ctxB, dlpB, outB)
		if err != nil {
			return err
		}
		m := models.AuditEvent{
			ID:                         uuid.New(),
			OrgID:                      ev.OrgID,
			ToolID:                     ev.ToolID,
			ToolName:                   ev.ToolName,
			RequestID:                  ev.RequestID,
			Actor:                      ev.Actor,
			ActorRole:                  ev.ActorRole,
			ActorScopes:                scopesB,
			InputRedacted:              inB,
			ContextRedacted:            ctxB,
			DLPSummary:                 dlpB,
			Decision:                   string(ev.Decision),
			PolicyID:                   ev.PolicyID,
			Reason:                     ev.Reason,
			Status:                     string(ev.Status),
			OutputRedacted:             outB,
			ErrorCode:                  ev.ErrorCode,
			ErrorMessage:               ev.ErrorMessage,
			LatencyMS:                  ev.LatencyMS,
			IdempotencyPresent:         ev.IdempotencyPresent,
			IdempotencyOutcome:         ev.IdempotencyOutcome,
			TimeoutMS:                  ev.TimeoutMS,
			BudgetRemainingMSAtExecute: ev.BudgetRemainingMSAtExecute,
			StageDurationsMS:           stageDurB,
			PrevEventHash:              prevHash,
			EventHash:                  &eventHash,
		}
		return tx.Create(&m).Error
	})
}

func (r *Repository) Query(ctx context.Context, orgID uuid.UUID, q auditdomain.Query) ([]auditdomain.AuditEvent, error) {
	tx := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if q.ToolName != nil && *q.ToolName != "" {
		tx = tx.Where("tool_name = ?", *q.ToolName)
	}
	if q.Decision != nil {
		tx = tx.Where("decision = ?", string(*q.Decision))
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", string(*q.Status))
	}
	if q.From != nil {
		tx = tx.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		tx = tx.Where("created_at <= ?", *q.To)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.AuditEvent
	orderBy := "created_at desc"
	if q.OrderAsc {
		orderBy = "created_at asc"
	}
	if err := tx.Order(orderBy).Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]auditdomain.AuditEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) StreamByFilters(ctx context.Context, orgID uuid.UUID, q auditdomain.Query, batchSize int, fn func(auditdomain.AuditEvent) error) error {
	txBase := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if q.ToolName != nil && *q.ToolName != "" {
		txBase = txBase.Where("tool_name = ?", *q.ToolName)
	}
	if q.Decision != nil {
		txBase = txBase.Where("decision = ?", string(*q.Decision))
	}
	if q.Status != nil {
		txBase = txBase.Where("status = ?", string(*q.Status))
	}
	if q.From != nil {
		txBase = txBase.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		txBase = txBase.Where("created_at <= ?", *q.To)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	emitted := 0
	var lastCreated *time.Time
	var lastID *uuid.UUID
	for emitted < limit {
		tx := txBase
		if lastCreated != nil && lastID != nil {
			tx = tx.Where("(created_at > ?) OR (created_at = ? AND id > ?)", *lastCreated, *lastCreated, *lastID)
		}
		chunkSize := batchSize
		if emitted+chunkSize > limit {
			chunkSize = limit - emitted
		}
		var rows []models.AuditEvent
		if err := tx.Order("created_at asc, id asc").Limit(chunkSize).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if err := fn(toDomain(row)); err != nil {
				return err
			}
			emitted++
			lastCreated = &row.CreatedAt
			id := row.ID
			lastID = &id
		}
	}
	return nil
}

func toDomain(m models.AuditEvent) auditdomain.AuditEvent {
	var in any
	_ = json.Unmarshal(m.InputRedacted, &in)
	var ctx any
	_ = json.Unmarshal(m.ContextRedacted, &ctx)
	var dlp any
	_ = json.Unmarshal(m.DLPSummary, &dlp)
	var scopes []string
	_ = json.Unmarshal(m.ActorScopes, &scopes)
	var stageDur map[string]int64
	_ = json.Unmarshal(m.StageDurationsMS, &stageDur)
	var out any
	if len(m.OutputRedacted) > 0 {
		_ = json.Unmarshal(m.OutputRedacted, &out)
	}
	return auditdomain.AuditEvent{
		ID:                         m.ID,
		OrgID:                      m.OrgID,
		ToolID:                     m.ToolID,
		ToolName:                   m.ToolName,
		RequestID:                  m.RequestID,
		Actor:                      m.Actor,
		ActorRole:                  m.ActorRole,
		ActorScopes:                scopes,
		InputRedacted:              in,
		ContextRedacted:            ctx,
		DLPSummary:                 dlp,
		Decision:                   auditdomain.Decision(m.Decision),
		PolicyID:                   m.PolicyID,
		Reason:                     m.Reason,
		Status:                     auditdomain.Status(m.Status),
		OutputRedacted:             out,
		ErrorCode:                  m.ErrorCode,
		ErrorMessage:               m.ErrorMessage,
		LatencyMS:                  m.LatencyMS,
		IdempotencyPresent:         m.IdempotencyPresent,
		IdempotencyOutcome:         m.IdempotencyOutcome,
		TimeoutMS:                  m.TimeoutMS,
		BudgetRemainingMSAtExecute: m.BudgetRemainingMSAtExecute,
		StageDurationsMS:           stageDur,
		PrevEventHash:              m.PrevEventHash,
		EventHash:                  m.EventHash,
		CreatedAt:                  m.CreatedAt,
	}
}

func computeEventHash(prevHash *string, ev auditdomain.AuditEvent, inB, ctxB, dlpB, outB []byte) (string, error) {
	prev := ""
	if prevHash != nil {
		prev = *prevHash
	}
	doc := map[string]any{
		"prev":                           prev,
		"org_id":                         ev.OrgID.String(),
		"tool_id":                        ev.ToolID.String(),
		"tool_name":                      ev.ToolName,
		"request_id":                     ev.RequestID,
		"decision":                       string(ev.Decision),
		"status":                         string(ev.Status),
		"policy_id":                      uuidPtrToString(ev.PolicyID),
		"reason":                         ptrToString(ev.Reason),
		"error_code":                     ptrToString(ev.ErrorCode),
		"error_msg":                      ptrToString(ev.ErrorMessage),
		"latency_ms":                     strconv.Itoa(ev.LatencyMS),
		"idem_present":                   ev.IdempotencyPresent,
		"idem_outcome":                   ev.IdempotencyOutcome,
		"timeout_ms":                     intPtrToString(ev.TimeoutMS),
		"budget_remaining_ms_at_execute": intPtrToString(ev.BudgetRemainingMSAtExecute),
		"actor_role":                     ptrToString(ev.ActorRole),
		"actor_scopes":                   ev.ActorScopes,
		"stage_durations_ms":             ev.StageDurationsMS,
		"input":                          string(inB),
		"context":                        string(ctxB),
		"dlp_summary":                    string(dlpB),
		"output":                         string(outB),
		"actor":                          ptrToString(ev.Actor),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func ptrToString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func uuidPtrToString(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func intPtrToString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
