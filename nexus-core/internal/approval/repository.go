package approval

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-core/internal/approval/repository/models"
	domain "nexus-core/internal/approval/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	inputJSON, _ := json.Marshal(req.InputRedacted)
	ctxJSON, _ := json.Marshal(req.ContextRedacted)
	row := models.PendingApproval{
		OrgID:              req.OrgID,
		ToolID:             req.ToolID,
		IntentID:           req.IntentID,
		ApprovalMode:       string(req.ApprovalMode),
		ApprovalGroupID:    req.ApprovalGroupID,
		ApprovalStep:       req.ApprovalStep,
		ApprovalStepsTotal: req.ApprovalStepsTotal,
		RequestID:          req.RequestID,
		ToolName:           req.ToolName,
		Actor:              req.Actor,
		Role:               req.Role,
		InputRedacted:      inputJSON,
		ContextRedacted:    ctxJSON,
		Reason:             req.Reason,
		PolicyID:           req.PolicyID,
		Status:             string(domain.StatusPending),
		ExpiresAt:          time.Now().Add(time.Duration(req.TTLSeconds) * time.Second),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.PendingApproval{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	var row models.PendingApproval
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).First(&row).Error; err != nil {
		return domain.PendingApproval{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []models.PendingApproval
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND status = ?", orgID, string(domain.StatusPending)).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.PendingApproval, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) ListByIntent(ctx context.Context, orgID, intentID uuid.UUID) ([]domain.PendingApproval, error) {
	var rows []models.PendingApproval
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND intent_id = ?", orgID, intentID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.PendingApproval, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Decide(ctx context.Context, orgID, id uuid.UUID, status domain.Status, decidedBy string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.PendingApproval{}).
		Where("id = ? AND org_id = ? AND status = ?", id, orgID, string(domain.StatusPending)).
		Updates(map[string]any{
			"status":     string(status),
			"decided_by": decidedBy,
			"decided_at": now,
		}).Error
}

func (r *Repository) ExpireOld(ctx context.Context) (int64, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.PendingApproval{}).
		Where("status = ? AND expires_at < ?", string(domain.StatusPending), time.Now()).
		Update("status", string(domain.StatusExpired))
	return tx.RowsAffected, tx.Error
}

func (r *Repository) RejectPendingByIntent(ctx context.Context, orgID, intentID uuid.UUID, decidedBy string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.PendingApproval{}).
		Where("org_id = ? AND intent_id = ? AND status = ?", orgID, intentID, string(domain.StatusPending)).
		Updates(map[string]any{
			"status":     string(domain.StatusRejected),
			"decided_by": decidedBy,
			"decided_at": now,
		}).Error
}

func toDomain(m models.PendingApproval) domain.PendingApproval {
	var input, ctxMap map[string]any
	_ = json.Unmarshal(m.InputRedacted, &input)
	_ = json.Unmarshal(m.ContextRedacted, &ctxMap)
	if input == nil {
		input = map[string]any{}
	}
	if ctxMap == nil {
		ctxMap = map[string]any{}
	}
	return domain.PendingApproval{
		ID:                 m.ID,
		OrgID:              m.OrgID,
		ToolID:             m.ToolID,
		IntentID:           m.IntentID,
		ApprovalMode:       domain.ApprovalMode(m.ApprovalMode),
		ApprovalGroupID:    m.ApprovalGroupID,
		ApprovalStep:       m.ApprovalStep,
		ApprovalStepsTotal: m.ApprovalStepsTotal,
		RequestID:          m.RequestID,
		ToolName:           m.ToolName,
		Actor:              m.Actor,
		Role:               m.Role,
		InputRedacted:      input,
		ContextRedacted:    ctxMap,
		Reason:             m.Reason,
		PolicyID:           m.PolicyID,
		Status:             domain.Status(m.Status),
		DecidedBy:          m.DecidedBy,
		DecidedAt:          m.DecidedAt,
		ExpiresAt:          m.ExpiresAt,
		CreatedAt:          m.CreatedAt,
	}
}
