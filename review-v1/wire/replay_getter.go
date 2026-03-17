package wire

import (
	"context"

	"github.com/google/uuid"
	"github.com/devpablocristo/nexus/review-v1/internal/audit"
	"github.com/devpablocristo/nexus/review-v1/internal/requests"
)

type replayRequestGetter struct {
	reqRepo requests.Repository
}

func newReplayRequestGetter(reqRepo requests.Repository) audit.RequestGetter {
	return &replayRequestGetter{reqRepo: reqRepo}
}

func (g *replayRequestGetter) GetReplayInfo(ctx context.Context, id uuid.UUID) (audit.ReplayRequestInfo, error) {
	req, err := g.reqRepo.GetByID(ctx, id)
	if err != nil {
		return audit.ReplayRequestInfo{}, err
	}
	return audit.ReplayRequestInfo{
		RequesterType:  string(req.RequesterType),
		RequesterID:    req.RequesterID,
		ActionType:     req.ActionType,
		TargetSystem:   req.TargetSystem,
		TargetResource: req.TargetResource,
		Status:         string(req.Status),
	}, nil
}
