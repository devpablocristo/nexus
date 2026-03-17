package wire

import (
	"context"

	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
	policydomain "github.com/devpablocristo/nexus/review-v1/internal/policies/usecases/domain"
	"github.com/devpablocristo/nexus/review-v1/internal/learning"
	"github.com/devpablocristo/nexus/review-v1/internal/policies"
)

type learningPolicyCreator struct {
	repo policies.Repository
}

func newLearningPolicyCreator(repo policies.Repository) learning.PolicyCreator {
	return &learningPolicyCreator{repo: repo}
}

func (c *learningPolicyCreator) CreateFromProposal(ctx context.Context, p learningdomain.PolicyProposal) (uuid.UUID, error) {
	policy := policydomain.Policy{
		Name:        p.ProposedName,
		Description: p.ProposedDescription,
		Expression:  p.ProposedExpression,
		Effect:      p.ProposedEffect,
		ActionType:  p.ProposedActionType,
		Priority:    p.ProposedPriority,
		Origin:      "learning",
		ProposalID:  &p.ID,
		Enabled:     true,
	}
	created, err := c.repo.Create(ctx, policy)
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}
