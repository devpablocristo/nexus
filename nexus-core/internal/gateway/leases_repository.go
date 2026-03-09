package gateway

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
)

type executionLeaseRow struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrgID           uuid.UUID      `gorm:"type:uuid;not null;index"`
	IntentID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	ToolName        string         `gorm:"not null"`
	RiskClass       string         `gorm:"not null"`
	Status          string         `gorm:"not null;default:'active'"`
	CredentialMode  string         `gorm:"not null;default:'none'"`
	CredentialHints datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	ExpiresAt       time.Time      `gorm:"not null;index"`
	UsedAt          *time.Time
	CreatedAt       time.Time `gorm:"autoCreateTime"`
}

func (executionLeaseRow) TableName() string { return "execution_leases" }

type LeaseRepository struct {
	db *gorm.DB
}

func NewLeaseRepository(db *gorm.DB) *LeaseRepository {
	return &LeaseRepository{db: db}
}

func (r *LeaseRepository) Create(ctx context.Context, lease gwdomain.ExecutionLease) (gwdomain.ExecutionLease, error) {
	hintsJSON, _ := json.Marshal(lease.CredentialHints)
	row := executionLeaseRow{
		OrgID:           lease.OrgID,
		IntentID:        lease.IntentID,
		ToolName:        lease.ToolName,
		RiskClass:       string(lease.RiskClass),
		Status:          string(lease.Status),
		CredentialMode:  lease.CredentialMode,
		CredentialHints: hintsJSON,
		ExpiresAt:       lease.ExpiresAt,
	}
	if row.Status == "" {
		row.Status = string(gwdomain.ExecutionLeaseStatusActive)
	}
	if row.CredentialMode == "" {
		row.CredentialMode = "none"
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return gwdomain.ExecutionLease{}, err
	}
	return toExecutionLease(row), nil
}

func (r *LeaseRepository) GetByID(ctx context.Context, orgID, leaseID uuid.UUID) (gwdomain.ExecutionLease, error) {
	var row executionLeaseRow
	if err := r.db.WithContext(ctx).Where("id = ? AND org_id = ?", leaseID, orgID).First(&row).Error; err != nil {
		return gwdomain.ExecutionLease{}, err
	}
	return toExecutionLease(row), nil
}

func (r *LeaseRepository) MarkUsed(ctx context.Context, orgID, leaseID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&executionLeaseRow{}).
		Where("id = ? AND org_id = ? AND status = ?", leaseID, orgID, string(gwdomain.ExecutionLeaseStatusActive)).
		Updates(map[string]any{
			"status":  string(gwdomain.ExecutionLeaseStatusUsed),
			"used_at": now,
		}).Error
}

func (r *LeaseRepository) MarkExpired(ctx context.Context, orgID, leaseID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&executionLeaseRow{}).
		Where("id = ? AND org_id = ? AND status = ?", leaseID, orgID, string(gwdomain.ExecutionLeaseStatusActive)).
		Update("status", string(gwdomain.ExecutionLeaseStatusExpired)).Error
}

func (r *LeaseRepository) MarkRevoked(ctx context.Context, orgID, leaseID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&executionLeaseRow{}).
		Where("id = ? AND org_id = ? AND status = ?", leaseID, orgID, string(gwdomain.ExecutionLeaseStatusActive)).
		Update("status", string(gwdomain.ExecutionLeaseStatusRevoked)).Error
}

func toExecutionLease(row executionLeaseRow) gwdomain.ExecutionLease {
	var hints map[string]any
	_ = json.Unmarshal(row.CredentialHints, &hints)
	if hints == nil {
		hints = map[string]any{}
	}
	return gwdomain.ExecutionLease{
		ID:              row.ID,
		OrgID:           row.OrgID,
		IntentID:        row.IntentID,
		ToolName:        row.ToolName,
		RiskClass:       gwdomain.RiskClass(row.RiskClass),
		Status:          gwdomain.ExecutionLeaseStatus(row.Status),
		CredentialMode:  row.CredentialMode,
		CredentialHints: hints,
		ExpiresAt:       row.ExpiresAt,
		UsedAt:          row.UsedAt,
		CreatedAt:       row.CreatedAt,
	}
}
