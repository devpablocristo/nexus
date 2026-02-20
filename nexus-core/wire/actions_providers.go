package wire

import (
	"context"

	"github.com/google/uuid"
	"github.com/google/wire"

	actiondomain "nexus-core/internal/actions/usecases/domain"
	"nexus-core/internal/actions"
	"nexus-core/internal/events"
	"nexus-core/internal/gateway"
)

func ProvideActionsEventSink(s events.Service) actions.EventSink { return s }

type gatewayActionOverridesAdapter struct {
	svc actions.Service
}

func (a gatewayActionOverridesAdapter) ResolveRuntimeOverrides(ctx context.Context, orgID uuid.UUID, toolName string) (gateway.RuntimeActionOverrides, error) {
	ov, err := a.svc.ResolveRuntimeOverrides(ctx, orgID, toolName)
	if err != nil {
		return gateway.RuntimeActionOverrides{}, err
	}
	return mapRuntimeOverrides(ov), nil
}

func mapRuntimeOverrides(ov actiondomain.RuntimeOverrides) gateway.RuntimeActionOverrides {
	return gateway.RuntimeActionOverrides{
		Deny:              ov.Deny,
		DenyReason:        ov.DenyReason,
		TenantRPMOverride: ov.TenantRPMOverride,
		ToolRPMOverride:   ov.ToolRPMOverride,
	}
}

func ProvideGatewayActionOverrides(s actions.Service) gateway.ActionOverridesPort {
	return gatewayActionOverridesAdapter{svc: s}
}

var ActionsSet = wire.NewSet(
	actions.NewRepository,
	wire.Bind(new(actions.RepositoryPort), new(*actions.Repository)),
	ProvideActionsEventSink,
	actions.NewService,
	actions.NewHandler,
	ProvideGatewayActionOverrides,
)
