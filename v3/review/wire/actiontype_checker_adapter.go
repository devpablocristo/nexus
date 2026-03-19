package wire

import (
	"context"

	"github.com/devpablocristo/nexus/v3/review/internal/actiontypes"
	"github.com/devpablocristo/nexus/v3/review/internal/requests"
)

// actionTypeCheckerAdapter adapta actiontypes.Usecases al port requests.ActionTypeChecker
type actionTypeCheckerAdapter struct {
	uc *actiontypes.Usecases
}

func newActionTypeCheckerAdapter(uc *actiontypes.Usecases) requests.ActionTypeChecker {
	return &actionTypeCheckerAdapter{uc: uc}
}

func (a *actionTypeCheckerAdapter) GetByName(ctx context.Context, name string) (requests.ActionTypeInfo, error) {
	at, err := a.uc.GetByName(ctx, name)
	if err != nil {
		return requests.ActionTypeInfo{}, err
	}
	return requests.ActionTypeInfo{
		Name:               at.Name,
		RiskClass:          string(at.RiskClass),
		RequiresBreakGlass: at.RequiresBreakGlass,
		Enabled:            at.Enabled,
	}, nil
}
