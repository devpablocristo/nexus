package actionengine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nexus-control-operators/internal/ops/actionengine/repository/models"
	actiondomain "nexus-control-operators/internal/ops/actionengine/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertCatalog(ctx context.Context, item actiondomain.CatalogItem) error {
	schemaRaw, _ := json.Marshal(item.Schema)
	row := models.CatalogItem{
		ActionType:       item.ActionType,
		SchemaJSON:       datatypes.JSON(schemaRaw),
		RequiresApproval: item.RequiresApproval,
		MaxTTLSeconds:    item.MaxTTLSeconds,
		Enabled:          item.Enabled,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "action_type"}},
		DoUpdates: clause.Assignments(map[string]any{
			"schema_json":        row.SchemaJSON,
			"requires_approval":  row.RequiresApproval,
			"max_ttl_seconds":    row.MaxTTLSeconds,
			"enabled":            row.Enabled,
			"updated_at":         gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

func (r *Repository) GetCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error) {
	var row models.CatalogItem
	if err := r.db.WithContext(ctx).Where("action_type = ? AND enabled = true", actionType).Take(&row).Error; err != nil {
		return actiondomain.CatalogItem{}, err
	}
	var schema map[string]any
	_ = json.Unmarshal(row.SchemaJSON, &schema)
	if schema == nil {
		schema = map[string]any{}
	}
	return actiondomain.CatalogItem{
		ActionType:       row.ActionType,
		Schema:           schema,
		RequiresApproval: row.RequiresApproval,
		MaxTTLSeconds:    row.MaxTTLSeconds,
		Enabled:          row.Enabled,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *Repository) CreateProposal(ctx context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error) {
	scopeRaw, _ := json.Marshal(in.Scope)
	paramsRaw, _ := json.Marshal(in.Params)
	refsRaw, _ := json.Marshal(in.EvidenceRefs)
	row := models.Proposal{
		ID:               uuid.New(),
		OrgID:            in.OrgID,
		IncidentID:       in.IncidentID,
		ActionType:       in.ActionType,
		ScopeJSON:        datatypes.JSON(scopeRaw),
		ParamsJSON:       datatypes.JSON(paramsRaw),
		TTLSeconds:       in.TTLSeconds,
		EvidenceRefsJSON: datatypes.JSON(refsRaw),
		IdempotencyKey:   in.IdempotencyKey,
		Status:           string(in.Status),
		ApprovalRequired: in.ApprovalRequired,
		ProposedBy:       in.ProposedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return actiondomain.Proposal{}, err
	}
	return toDomainProposal(row), nil
}

func (r *Repository) GetProposalByID(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Proposal, error) {
	var row models.Proposal
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, proposalID).Take(&row).Error; err != nil {
		return actiondomain.Proposal{}, err
	}
	return toDomainProposal(row), nil
}

func (r *Repository) GetProposalByIdempotencyKey(ctx context.Context, orgID uuid.UUID, idempotencyKey string) (actiondomain.Proposal, error) {
	var row models.Proposal
	err := r.db.WithContext(ctx).Where("org_id = ? AND idempotency_key = ?", orgID, idempotencyKey).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return actiondomain.Proposal{}, gorm.ErrRecordNotFound
		}
		return actiondomain.Proposal{}, err
	}
	return toDomainProposal(row), nil
}

func (r *Repository) UpdateProposalStatus(ctx context.Context, orgID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error) {
	if err := r.db.WithContext(ctx).Model(&models.Proposal{}).
		Where("org_id = ? AND id = ?", orgID, proposalID).
		Updates(map[string]any{"status": string(status), "updated_at": gorm.Expr("now()")}).
		Error; err != nil {
		return actiondomain.Proposal{}, err
	}
	return r.GetProposalByID(ctx, orgID, proposalID)
}

func (r *Repository) CreateExecution(ctx context.Context, in actiondomain.Execution) (actiondomain.Execution, error) {
	outputRaw, _ := json.Marshal(in.Output)
	row := models.Execution{
		ID:           uuid.New(),
		ProposalID:   in.ProposalID,
		OrgID:        in.OrgID,
		Mode:         string(in.Mode),
		Status:       string(in.Status),
		ErrorCode:    in.ErrorCode,
		ErrorMessage: in.ErrorMessage,
		OutputJSON:   datatypes.JSON(outputRaw),
		ExecutedBy:   in.ExecutedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return actiondomain.Execution{}, err
	}
	return toDomainExecution(row), nil
}

func (r *Repository) CreateApproval(ctx context.Context, in actiondomain.Approval) (actiondomain.Approval, error) {
	row := models.Approval{
		ID:         uuid.New(),
		ProposalID: in.ProposalID,
		OrgID:      in.OrgID,
		Approved:   in.Approved,
		Approver:   in.Approver,
		Comment:    in.Comment,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return actiondomain.Approval{}, err
	}
	return toDomainApproval(row), nil
}

func (r *Repository) GetLatestApproval(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Approval, error) {
	var row models.Approval
	if err := r.db.WithContext(ctx).Where("org_id = ? AND proposal_id = ?", orgID, proposalID).Order("created_at desc").Take(&row).Error; err != nil {
		return actiondomain.Approval{}, err
	}
	return toDomainApproval(row), nil
}

func toDomainProposal(m models.Proposal) actiondomain.Proposal {
	var scope map[string]any
	_ = json.Unmarshal(m.ScopeJSON, &scope)
	if scope == nil {
		scope = map[string]any{}
	}
	var params map[string]any
	_ = json.Unmarshal(m.ParamsJSON, &params)
	if params == nil {
		params = map[string]any{}
	}
	var refs []string
	_ = json.Unmarshal(m.EvidenceRefsJSON, &refs)
	return actiondomain.Proposal{
		ID:               m.ID,
		OrgID:            m.OrgID,
		IncidentID:       m.IncidentID,
		ActionType:       m.ActionType,
		Scope:            scope,
		Params:           params,
		TTLSeconds:       m.TTLSeconds,
		EvidenceRefs:     refs,
		IdempotencyKey:   m.IdempotencyKey,
		Status:           actiondomain.ProposalStatus(m.Status),
		ApprovalRequired: m.ApprovalRequired,
		ProposedBy:       m.ProposedBy,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func toDomainExecution(m models.Execution) actiondomain.Execution {
	var output map[string]any
	_ = json.Unmarshal(m.OutputJSON, &output)
	if output == nil {
		output = map[string]any{}
	}
	return actiondomain.Execution{
		ID:           m.ID,
		ProposalID:   m.ProposalID,
		OrgID:        m.OrgID,
		Mode:         actiondomain.ExecutionMode(m.Mode),
		Status:       actiondomain.ExecutionStatus(m.Status),
		ErrorCode:    m.ErrorCode,
		ErrorMessage: m.ErrorMessage,
		Output:       output,
		ExecutedBy:   m.ExecutedBy,
		ExecutedAt:   m.ExecutedAt,
	}
}

func toDomainApproval(m models.Approval) actiondomain.Approval {
	return actiondomain.Approval{
		ID:         m.ID,
		ProposalID: m.ProposalID,
		OrgID:      m.OrgID,
		Approved:   m.Approved,
		Approver:   m.Approver,
		Comment:    m.Comment,
		CreatedAt:  m.CreatedAt,
	}
}
