package policyproposal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"control-plane/internal/policyproposal/repository/models"
	proposaldomain "control-plane/internal/policyproposal/usecases/domain"
	"nexus/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in proposaldomain.Proposal) (proposaldomain.Proposal, error) {
	diff, _ := json.Marshal(in.Diff)
	tests, _ := json.Marshal(in.TestsSuggested)
	row := models.Proposal{
		ID:             uuid.New(),
		OrgID:          in.OrgID,
		Status:         string(in.Status),
		Diff:           diff,
		Rationale:      in.Rationale,
		TestsSuggested: tests,
		RollbackPlan:   in.RollbackPlan,
		CreatedBy:      in.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return proposaldomain.Proposal{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]proposaldomain.Proposal, error) {
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
	var rows []models.Proposal
	if err := q.Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]proposaldomain.Proposal, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (proposaldomain.Proposal, error) {
	var row models.Proposal
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return proposaldomain.Proposal{}, types.NewHTTPError(404, types.ErrCodeNotFound, "proposal not found")
		}
		return proposaldomain.Proposal{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) UpdateDecision(ctx context.Context, orgID, id uuid.UUID, status proposaldomain.Status, decidedBy *string, decidedAt time.Time) (proposaldomain.Proposal, error) {
	updates := map[string]any{
		"status":     string(status),
		"decided_at": decidedAt,
	}
	if decidedBy != nil {
		updates["decided_by"] = *decidedBy
	}
	if err := r.db.WithContext(ctx).Model(&models.Proposal{}).
		Where("org_id = ? AND id = ?", orgID, id).
		Updates(updates).Error; err != nil {
		return proposaldomain.Proposal{}, err
	}
	return r.GetByID(ctx, orgID, id)
}

func (r *Repository) CreateVersion(ctx context.Context, orgID uuid.UUID, proposalID *uuid.UUID, label string, spec map[string]any, mode string, createdBy *string) error {
	b, _ := json.Marshal(spec)
	row := models.PolicyVersion{
		ID:         uuid.New(),
		OrgID:      orgID,
		ProposalID: proposalID,
		Label:      label,
		Spec:       b,
		Mode:       mode,
		CreatedBy:  createdBy,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func toDomain(m models.Proposal) proposaldomain.Proposal {
	var diff map[string]any
	_ = json.Unmarshal(m.Diff, &diff)
	if diff == nil {
		diff = map[string]any{}
	}
	var tests []string
	_ = json.Unmarshal(m.TestsSuggested, &tests)
	return proposaldomain.Proposal{
		ID:             m.ID,
		OrgID:          m.OrgID,
		Status:         proposaldomain.Status(m.Status),
		Diff:           diff,
		Rationale:      m.Rationale,
		TestsSuggested: tests,
		RollbackPlan:   m.RollbackPlan,
		CreatedBy:      m.CreatedBy,
		CreatedAt:      m.CreatedAt,
		DecidedBy:      m.DecidedBy,
		DecidedAt:      m.DecidedAt,
	}
}
