package actionengine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	actiondomain "nexus-core/internal/ops/actionengine/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	tenantdomain "nexus-core/internal/ops/tenant/usecases/domain"
	"nexus-core/pkg/types"
	"nexus-core/pkg/validations/jsonschema"
)

func TestEngine_DryRun_IdempotencyIgnoresTTL(t *testing.T) {
	t.Parallel()

	repo := newInMemoryRepo()
	eng := NewEngine(
		repo,
		&noopEmitter{},
		tenantStub{maxTTL: 1800},
		EngineConfig{ActionSchemaDir: filepath.Join(repoRoot(t), "internal", "ops", "schemas", "actions")},
		jsonschema.NewCompilerCache(),
	)
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := uuid.MustParse("f503f46f-c137-4165-b9ca-999d0d6f328f")

	first, err := eng.DryRun(context.Background(), orgID, ptr("alice"), EngineRequest{
		IncidentID: &incidentID,
		ActionType: "set_rate_limit",
		Scope: map[string]any{
			"level":   "tool",
			"org_id":  orgID.String(),
			"tool_id": "world.move",
		},
		TTLSeconds: 600,
		Params: map[string]any{
			"rpm":     120,
			"tool_id": "world.move",
		},
	})
	if err != nil {
		t.Fatalf("dry-run first failed: %v", err)
	}

	second, err := eng.DryRun(context.Background(), orgID, ptr("alice"), EngineRequest{
		IncidentID: &incidentID,
		ActionType: "set_rate_limit",
		Scope: map[string]any{
			"level":   "tool",
			"org_id":  orgID.String(),
			"tool_id": "world.move",
		},
		TTLSeconds: 1200,
		Params: map[string]any{
			"rpm":     120,
			"tool_id": "world.move",
		},
	})
	if err != nil {
		t.Fatalf("dry-run second failed: %v", err)
	}

	if !second.Replay {
		t.Fatalf("expected second call to be replay")
	}
	if first.Proposal.ID != second.Proposal.ID {
		t.Fatalf("expected same proposal id on replay")
	}
}

func TestEngine_Apply_RequiresApproval(t *testing.T) {
	t.Parallel()

	repo := newInMemoryRepo()
	eng := NewEngine(
		repo,
		&noopEmitter{},
		tenantStub{maxTTL: 1800},
		EngineConfig{ActionSchemaDir: filepath.Join(repoRoot(t), "internal", "ops", "schemas", "actions")},
		jsonschema.NewCompilerCache(),
	)
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := uuid.MustParse("f503f46f-c137-4165-b9ca-999d0d6f328f")

	dryRun, err := eng.DryRun(context.Background(), orgID, ptr("bob"), EngineRequest{
		IncidentID: &incidentID,
		ActionType: "quarantine_tenant",
		Scope: map[string]any{
			"level":  "org",
			"org_id": orgID.String(),
		},
		TTLSeconds: 300,
		Params: map[string]any{
			"org_id": orgID.String(),
			"mode":   "soft",
		},
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if !dryRun.ApprovalRequired {
		t.Fatalf("expected approval_required")
	}

	_, err = eng.Apply(context.Background(), orgID, ptr("bob"), EngineRequest{
		ProposalID: proposalIDPtr(dryRun.Proposal),
	})
	if err == nil {
		t.Fatalf("expected apply to fail without approval")
	}
	var he types.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.Code != types.ErrCodeApprovalRequired {
		t.Fatalf("unexpected error code: got=%s want=%s", he.Code, types.ErrCodeApprovalRequired)
	}
}

type inMemoryRepo struct {
	catalog    map[string]actiondomain.CatalogItem
	proposals  map[uuid.UUID]actiondomain.Proposal
	proposalsByKey map[string]uuid.UUID
	executions map[uuid.UUID]actiondomain.Execution
	approvals  map[uuid.UUID][]actiondomain.Approval
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		catalog:        map[string]actiondomain.CatalogItem{},
		proposals:      map[uuid.UUID]actiondomain.Proposal{},
		proposalsByKey: map[string]uuid.UUID{},
		executions:     map[uuid.UUID]actiondomain.Execution{},
		approvals:      map[uuid.UUID][]actiondomain.Approval{},
	}
}

func (r *inMemoryRepo) UpsertCatalog(_ context.Context, item actiondomain.CatalogItem) error {
	r.catalog[item.ActionType] = item
	return nil
}

func (r *inMemoryRepo) GetCatalog(_ context.Context, actionType string) (actiondomain.CatalogItem, error) {
	item, ok := r.catalog[actionType]
	if !ok {
		return actiondomain.CatalogItem{}, gorm.ErrRecordNotFound
	}
	return item, nil
}

func (r *inMemoryRepo) CreateProposal(_ context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error) {
	in.ID = uuid.New()
	r.proposals[in.ID] = in
	r.proposalsByKey[in.IdempotencyKey] = in.ID
	return in, nil
}

func (r *inMemoryRepo) GetProposalByID(_ context.Context, _ uuid.UUID, proposalID uuid.UUID) (actiondomain.Proposal, error) {
	p, ok := r.proposals[proposalID]
	if !ok {
		return actiondomain.Proposal{}, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (r *inMemoryRepo) GetProposalByIdempotencyKey(_ context.Context, _ uuid.UUID, key string) (actiondomain.Proposal, error) {
	id, ok := r.proposalsByKey[key]
	if !ok {
		return actiondomain.Proposal{}, gorm.ErrRecordNotFound
	}
	return r.proposals[id], nil
}

func (r *inMemoryRepo) UpdateProposalStatus(_ context.Context, _ uuid.UUID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error) {
	p, ok := r.proposals[proposalID]
	if !ok {
		return actiondomain.Proposal{}, gorm.ErrRecordNotFound
	}
	p.Status = status
	r.proposals[proposalID] = p
	return p, nil
}

func (r *inMemoryRepo) CreateExecution(_ context.Context, in actiondomain.Execution) (actiondomain.Execution, error) {
	in.ID = uuid.New()
	r.executions[in.ID] = in
	return in, nil
}

func (r *inMemoryRepo) CreateApproval(_ context.Context, in actiondomain.Approval) (actiondomain.Approval, error) {
	in.ID = uuid.New()
	r.approvals[in.ProposalID] = append(r.approvals[in.ProposalID], in)
	return in, nil
}

func (r *inMemoryRepo) GetLatestApproval(_ context.Context, _ uuid.UUID, proposalID uuid.UUID) (actiondomain.Approval, error) {
	items := r.approvals[proposalID]
	if len(items) == 0 {
		return actiondomain.Approval{}, gorm.ErrRecordNotFound
	}
	return items[len(items)-1], nil
}

type tenantStub struct {
	maxTTL int
}

func (t tenantStub) GetProfile(_ context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error) {
	return tenantdomain.TenantProfile{
		OrgID:         orgID,
		MaxTTLSeconds: t.maxTTL,
	}, nil
}

type noopEmitter struct{}

func (*noopEmitter) Emit(context.Context, opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	return opsdomain.StoredEvent{}, nil
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	cur := wd
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			t.Fatalf("go.mod not found from %s", wd)
		}
		cur = next
	}
}

func ptr(v string) *string {
	return &v
}

func proposalIDPtr(p actiondomain.Proposal) *uuid.UUID {
	id := p.ID
	return &id
}
