package actions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-core/internal/actions/repository/models"
	actiondomain "nexus-core/internal/actions/usecases/domain"
	"nexus/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, a actiondomain.Action) (actiondomain.Action, error) {
	params, _ := json.Marshal(a.Params)
	refs, _ := json.Marshal(a.EvidenceRefs)
	row := models.Action{
		ID:           uuid.New(),
		OrgID:        a.OrgID,
		ScopeType:    string(a.ScopeType),
		ScopeID:      a.ScopeID,
		ActionType:   string(a.ActionType),
		Params:       params,
		TTLSeconds:   a.TTLSeconds,
		Status:       string(a.Status),
		EvidenceRefs: refs,
		CreatedBy:    a.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return actiondomain.Action{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (actiondomain.Action, error) {
	var row models.Action
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return actiondomain.Action{}, types.NewHTTPError(404, types.ErrCodeNotFound, "action not found")
		}
		return actiondomain.Action{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, status, actionType string, limit int) ([]actiondomain.Action, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q := r.db.WithContext(ctx).Where("org_id = ?", orgID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if actionType != "" {
		q = q.Where("action_type = ?", actionType)
	}
	var rows []models.Action
	if err := q.Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]actiondomain.Action, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status actiondomain.Status, rolledBackBy *string, rolledBackAt *time.Time) (actiondomain.Action, error) {
	updates := map[string]any{"status": string(status)}
	if rolledBackBy != nil {
		updates["rolled_back_by"] = *rolledBackBy
	}
	if rolledBackAt != nil {
		updates["rolled_back_at"] = *rolledBackAt
	}
	if err := r.db.WithContext(ctx).Model(&models.Action{}).
		Where("org_id = ? AND id = ?", orgID, id).
		Updates(updates).Error; err != nil {
		return actiondomain.Action{}, err
	}
	return r.GetByID(ctx, orgID, id)
}

func (r *Repository) ListExpiredCandidates(ctx context.Context, now time.Time, limit int) ([]actiondomain.Action, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []models.Action
	if err := r.db.WithContext(ctx).
		Where("status = ?", string(actiondomain.StatusActive)).
		Where("ttl_seconds > 0").
		Where("created_at + (ttl_seconds * interval '1 second') <= ?", now).
		Order("created_at asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]actiondomain.Action, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) ListActiveForRun(ctx context.Context, orgID uuid.UUID, toolName string, now time.Time) ([]actiondomain.Action, error) {
	var rows []models.Action
	err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Where("status = ?", string(actiondomain.StatusActive)).
		Where("ttl_seconds <= 0 OR created_at + (ttl_seconds * interval '1 second') > ?", now).
		Where("scope_type = ? OR scope_type = ? OR (scope_type = ? AND scope_id = ?)", string(actiondomain.ScopeGlobal), string(actiondomain.ScopeTenant), string(actiondomain.ScopeTool), toolName).
		Order("created_at asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]actiondomain.Action, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.Action) actiondomain.Action {
	var params map[string]any
	_ = json.Unmarshal(m.Params, &params)
	if params == nil {
		params = map[string]any{}
	}
	var refs []string
	_ = json.Unmarshal(m.EvidenceRefs, &refs)
	return actiondomain.Action{
		ID:           m.ID,
		OrgID:        m.OrgID,
		ScopeType:    actiondomain.ScopeType(m.ScopeType),
		ScopeID:      m.ScopeID,
		ActionType:   actiondomain.ActionType(m.ActionType),
		Params:       params,
		TTLSeconds:   m.TTLSeconds,
		Status:       actiondomain.Status(m.Status),
		EvidenceRefs: refs,
		CreatedAt:    m.CreatedAt,
		CreatedBy:    m.CreatedBy,
		RolledBackAt: m.RolledBackAt,
		RolledBackBy: m.RolledBackBy,
	}
}
