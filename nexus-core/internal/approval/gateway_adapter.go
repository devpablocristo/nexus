package approval

import (
	"context"

	"nexus-core/internal/approval/usecases/domain"
	"nexus-core/internal/gateway"
)

type GatewayAdapter struct {
	uc *Usecases
}

func NewGatewayAdapter(uc *Usecases) *GatewayAdapter {
	return &GatewayAdapter{uc: uc}
}

func (a *GatewayAdapter) RequestApproval(ctx context.Context, req gateway.ApprovalRequest) (string, error) {
	pa, err := a.uc.RequestApproval(ctx, domain.CreateRequest{
		OrgID:           req.OrgID,
		ToolID:          req.ToolID,
		IntentID:        req.IntentID,
		RequestID:       req.RequestID,
		ToolName:        req.ToolName,
		Actor:           req.Actor,
		Role:            req.Role,
		InputRedacted:   req.InputRedacted,
		ContextRedacted: req.ContextRedacted,
		Reason:          req.Reason,
		PolicyID:        req.PolicyID,
		TTLSeconds:      req.TTLSeconds,
	})
	if err != nil {
		return "", err
	}
	return pa.ID.String(), nil
}

var _ gateway.ApprovalPort = (*GatewayAdapter)(nil)
