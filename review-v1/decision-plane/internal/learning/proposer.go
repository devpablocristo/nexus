package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
)

// PolicyProposer genera propuestas de políticas a partir de patrones detectados.
type PolicyProposer interface {
	GenerateProposal(ctx context.Context, pattern Pattern) (*learningdomain.PolicyProposal, error)
}

// StubProposer genera propuestas sin usar IA (basado en templates).
type StubProposer struct{}

func NewStubProposer() *StubProposer {
	return &StubProposer{}
}

func (s *StubProposer) GenerateProposal(ctx context.Context, pattern Pattern) (*learningdomain.PolicyProposal, error) {
	now := time.Now().UTC()
	name := fmt.Sprintf("auto-approve-%s", pattern.ActionType)
	description := fmt.Sprintf(
		"Auto-approve %s — historically approved %.0f%% of the time (%d/%d)",
		pattern.ActionType, pattern.ApprovalRate*100, pattern.Approved, pattern.Total,
	)
	expression := fmt.Sprintf("request.action_type == '%s'", pattern.ActionType)
	summary := fmt.Sprintf(
		"En los últimos %s, %.0f%% de las requests '%s' fueron aprobadas (%d de %d).",
		pattern.TimeWindow, pattern.ApprovalRate*100, pattern.ActionType, pattern.Approved, pattern.Total,
	)

	return &learningdomain.PolicyProposal{
		ID:                  uuid.New(),
		ProposedName:        name,
		ProposedDescription: description,
		ProposedExpression:  expression,
		ProposedEffect:      "allow",
		ProposedActionType:  &pattern.ActionType,
		ProposedPriority:    100,
		PatternSummary:      summary,
		Confidence:          pattern.ApprovalRate,
		SampleSize:          pattern.Total,
		TimeWindow:          pattern.TimeWindow,
		Status:              learningdomain.ProposalStatusPending,
		CreatedAt:           now,
	}, nil
}
