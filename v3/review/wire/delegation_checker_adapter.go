package wire

import (
	"context"

	"github.com/devpablocristo/nexus/v3/review/internal/delegations"
	"github.com/devpablocristo/nexus/v3/review/internal/requests"
)

// delegationCheckerAdapter adapta delegations.Usecases al port requests.DelegationChecker
type delegationCheckerAdapter struct {
	uc *delegations.Usecases
}

func newDelegationCheckerAdapter(uc *delegations.Usecases) requests.DelegationChecker {
	return &delegationCheckerAdapter{uc: uc}
}

func (a *delegationCheckerAdapter) CheckDelegation(ctx context.Context, agentID, actionType string) (bool, error) {
	allowed, _, err := a.uc.CheckDelegation(ctx, agentID, actionType)
	return allowed, err
}
