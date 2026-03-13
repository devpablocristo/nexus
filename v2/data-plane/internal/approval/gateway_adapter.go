package approval

import (
	"context"

	"nexus/v2/data-plane/internal/gateway"

	domain "nexus/v2/data-plane/internal/approval/usecases/domain"
)

type GatewayAdapter struct {
	uc *Usecases
}

func NewGatewayAdapter(uc *Usecases) *GatewayAdapter {
	return &GatewayAdapter{uc: uc}
}

func (a *GatewayAdapter) RequestApproval(ctx context.Context, req gateway.ApprovalRequest) (string, error) {
	item, err := a.uc.RequestApproval(ctx, domain.CreateRequest{
		IntentID:   req.IntentID,
		RequestID:  req.RequestID,
		ToolName:   req.ToolName,
		Reason:     req.Reason,
		TTLSeconds: req.TTLSeconds,
	})
	if err != nil {
		return "", err
	}
	return item.ID.String(), nil
}
