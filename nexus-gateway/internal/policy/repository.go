package policy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-gateway/internal/policy/repository/models"
	policydomain "nexus-gateway/internal/policy/usecases/domain"
	"nexus-gateway/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, orgID uuid.UUID, p policydomain.Policy) (policydomain.Policy, error) {
	m := models.Policy{
		ID:             uuid.New(),
		OrgID:          orgID,
		ToolID:         p.ToolID,
		Effect:         string(p.Effect),
		Priority:       p.Priority,
		ConditionsJSON: p.ConditionsJSON,
		LimitsJSON:     p.LimitsJSON,
		ReasonTemplate: p.ReasonTemplate,
		Enabled:        p.Enabled,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return policydomain.Policy{}, err
	}
	return toDomain(m), nil
}

func (r *Repository) ListByToolID(ctx context.Context, orgID, toolID uuid.UUID) ([]policydomain.Policy, error) {
	var rows []models.Policy
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND tool_id = ? AND enabled = true", orgID, toolID).
		Order("priority asc, created_at asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]policydomain.Policy, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, policyID uuid.UUID) (policydomain.Policy, error) {
	var row models.Policy
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, policyID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return policydomain.Policy{}, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "policy not found")
		}
		return policydomain.Policy{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) Update(ctx context.Context, orgID uuid.UUID, policyID uuid.UUID, patch PolicyPatch) (policydomain.Policy, error) {
	var row models.Policy
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, policyID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return policydomain.Policy{}, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "policy not found")
		}
		return policydomain.Policy{}, err
	}
	if patch.Effect != nil {
		row.Effect = *patch.Effect
	}
	if patch.Priority != nil {
		row.Priority = *patch.Priority
	}
	if patch.ReasonTemplate != nil {
		row.ReasonTemplate = *patch.ReasonTemplate
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	if patch.Conditions != nil {
		b, _ := json.Marshal(*patch.Conditions)
		row.ConditionsJSON = b
	}
	if patch.Limits != nil {
		b, _ := json.Marshal(*patch.Limits)
		row.LimitsJSON = b
	}
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return policydomain.Policy{}, err
	}
	return toDomain(row), nil
}

func toDomain(m models.Policy) policydomain.Policy {
	return policydomain.Policy{
		ID:             m.ID,
		OrgID:          m.OrgID,
		ToolID:         m.ToolID,
		Effect:         policydomain.Effect(m.Effect),
		Priority:       m.Priority,
		ConditionsJSON: []byte(m.ConditionsJSON),
		LimitsJSON:     []byte(m.LimitsJSON),
		ReasonTemplate: m.ReasonTemplate,
		Enabled:        m.Enabled,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
