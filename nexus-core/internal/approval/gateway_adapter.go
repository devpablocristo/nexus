package approval

import (
	"context"

	"nexus-core/internal/approval/usecases/domain"
	"nexus-core/internal/gateway"
)

// GatewayAdapter adapts the approval.Service to the gateway.ApprovalPort interface.
type GatewayAdapter struct {
	svc *Service
}

func NewGatewayAdapter(svc *Service) *GatewayAdapter {
	return &GatewayAdapter{svc: svc}
}

func (a *GatewayAdapter) RequestApproval(ctx context.Context, req gateway.ApprovalRequest) (string, error) {
	pa, err := a.svc.RequestApproval(ctx, domain.CreateRequest{
		OrgID:           req.OrgID,
		ToolID:          req.ToolID,
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
