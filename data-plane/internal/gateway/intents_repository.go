package gateway

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	gwdomain "data-plane/internal/gateway/usecases/domain"
)

type executionIntentRow struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrgID                uuid.UUID `gorm:"type:uuid;not null"`
	ToolID               uuid.UUID `gorm:"type:uuid;not null"`
	ToolName             string    `gorm:"not null"`
	RequestID            string    `gorm:"not null"`
	Actor                *string
	Role                 *string
	ScopesJSON           datatypes.JSON `gorm:"type:jsonb;default:'[]'"`
	InputPayload         datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	ContextPayload       datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	PolicyID             *uuid.UUID     `gorm:"type:uuid"`
	RiskClass            string         `gorm:"not null"`
	Reason               string         `gorm:"default:''"`
	ApprovalID           *uuid.UUID     `gorm:"type:uuid"`
	Status               string         `gorm:"default:'pending_approval'"`
	PreflightStatus      string         `gorm:"default:'not_required'"`
	PreflightSummary     datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	PreflightArtifactSHA *string
	PreflightCompletedAt *time.Time
	ExpiresAt            time.Time `gorm:"not null"`
	ApprovedAt           *time.Time
	ExecutedAt           *time.Time
	CreatedAt            time.Time `gorm:"autoCreateTime"`
	UpdatedAt            time.Time `gorm:"autoUpdateTime"`
}

func (executionIntentRow) TableName() string { return "execution_intents" }

type IntentRepository struct {
	db *gorm.DB
}

func NewIntentRepository(db *gorm.DB) *IntentRepository {
	return &IntentRepository{db: db}
}

func (r *IntentRepository) Create(ctx context.Context, intent gwdomain.ExecutionIntent) (gwdomain.ExecutionIntent, error) {
	scopesJSON, _ := json.Marshal(intent.Scopes)
	inputJSON, _ := json.Marshal(intent.Input)
	contextJSON, _ := json.Marshal(intent.Context)
	preflightSummaryJSON, _ := json.Marshal(intent.PreflightSummary)
	row := executionIntentRow{
		OrgID:                intent.OrgID,
		ToolID:               intent.ToolID,
		ToolName:             intent.ToolName,
		RequestID:            intent.RequestID,
		Actor:                intent.Actor,
		Role:                 intent.Role,
		ScopesJSON:           scopesJSON,
		InputPayload:         inputJSON,
		ContextPayload:       contextJSON,
		PolicyID:             intent.PolicyID,
		RiskClass:            string(intent.RiskClass),
		Reason:               intent.Reason,
		Status:               string(intent.Status),
		PreflightStatus:      string(intent.PreflightStatus),
		PreflightSummary:     preflightSummaryJSON,
		PreflightArtifactSHA: intent.PreflightArtifactSHA,
		PreflightCompletedAt: intent.PreflightCompletedAt,
		ExpiresAt:            intent.ExpiresAt,
	}
	if row.Status == "" {
		row.Status = string(gwdomain.IntentStatusPendingApproval)
	}
	if row.PreflightStatus == "" {
		row.PreflightStatus = string(gwdomain.PreflightStatusNotRequired)
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return gwdomain.ExecutionIntent{}, err
	}
	return toExecutionIntent(row), nil
}

func (r *IntentRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (gwdomain.ExecutionIntent, error) {
	var row executionIntentRow
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).First(&row).Error; err != nil {
		return gwdomain.ExecutionIntent{}, err
	}
	return toExecutionIntent(row), nil
}

func (r *IntentRepository) ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]gwdomain.ExecutionIntent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []executionIntentRow
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]gwdomain.ExecutionIntent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toExecutionIntent(row))
	}
	return out, nil
}

func (r *IntentRepository) LinkApproval(ctx context.Context, orgID, intentID, approvalID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&executionIntentRow{}).
		Where("id = ? AND org_id = ?", intentID, orgID).
		Updates(map[string]any{
			"approval_id": approvalID,
			"updated_at":  time.Now(),
		}).Error
}

func (r *IntentRepository) MarkIntentApproved(ctx context.Context, orgID, intentID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&executionIntentRow{}).
		Where("id = ? AND org_id = ? AND status = ?", intentID, orgID, string(gwdomain.IntentStatusPendingApproval)).
		Updates(map[string]any{
			"status":      string(gwdomain.IntentStatusApproved),
			"approved_at": now,
			"updated_at":  now,
		}).Error
}

func (r *IntentRepository) MarkIntentRejected(ctx context.Context, orgID, intentID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&executionIntentRow{}).
		Where("id = ? AND org_id = ? AND status = ?", intentID, orgID, string(gwdomain.IntentStatusPendingApproval)).
		Updates(map[string]any{
			"status":     string(gwdomain.IntentStatusRejected),
			"updated_at": now,
		}).Error
}

func (r *IntentRepository) MarkExecuted(ctx context.Context, orgID, intentID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&executionIntentRow{}).
		Where("id = ? AND org_id = ? AND status = ?", intentID, orgID, string(gwdomain.IntentStatusApproved)).
		Updates(map[string]any{
			"status":      string(gwdomain.IntentStatusExecuted),
			"executed_at": now,
			"updated_at":  now,
		}).Error
}

func toExecutionIntent(row executionIntentRow) gwdomain.ExecutionIntent {
	var scopes []string
	var input map[string]any
	var contextMap map[string]any
	var preflightSummary map[string]any
	_ = json.Unmarshal(row.ScopesJSON, &scopes)
	_ = json.Unmarshal(row.InputPayload, &input)
	_ = json.Unmarshal(row.ContextPayload, &contextMap)
	_ = json.Unmarshal(row.PreflightSummary, &preflightSummary)
	if scopes == nil {
		scopes = []string{}
	}
	if input == nil {
		input = map[string]any{}
	}
	if contextMap == nil {
		contextMap = map[string]any{}
	}
	if preflightSummary == nil {
		preflightSummary = map[string]any{}
	}
	return gwdomain.ExecutionIntent{
		ID:                   row.ID,
		OrgID:                row.OrgID,
		ToolID:               row.ToolID,
		ToolName:             row.ToolName,
		RequestID:            row.RequestID,
		Actor:                row.Actor,
		Role:                 row.Role,
		Scopes:               scopes,
		Input:                input,
		Context:              contextMap,
		PolicyID:             row.PolicyID,
		RiskClass:            gwdomain.RiskClass(row.RiskClass),
		Reason:               row.Reason,
		ApprovalID:           row.ApprovalID,
		Status:               gwdomain.IntentStatus(row.Status),
		PreflightStatus:      gwdomain.PreflightStatus(row.PreflightStatus),
		PreflightSummary:     preflightSummary,
		PreflightArtifactSHA: row.PreflightArtifactSHA,
		PreflightCompletedAt: row.PreflightCompletedAt,
		ExpiresAt:            row.ExpiresAt,
		ApprovedAt:           row.ApprovedAt,
		ExecutedAt:           row.ExecutedAt,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}
