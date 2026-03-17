package learning

import (
	"context"

	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
)

type Repository interface {
	CreateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error)
	ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error)
	GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error)
	UpdateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error)
}

type PatternAnalyzer interface {
	Analyze(ctx context.Context, timeWindowDays int, minSampleSize int, minApprovalRate float64) ([]Pattern, error)
}

type Pattern struct {
	ActionType    string
	Total         int
	Approved      int
	ApprovalRate  float64
	TimeWindow    string
}

// PolicyCreator creates a policy from an accepted proposal (injected from wire).
type PolicyCreator interface {
	CreateFromProposal(ctx context.Context, p learningdomain.PolicyProposal) (policyID uuid.UUID, err error)
}
