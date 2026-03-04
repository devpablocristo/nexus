package mitigation

import (
	"context"
	"strings"

	"nexus-control-operators/internal/agents/mitigation/worker"
	opsaction "nexus-control-operators/internal/ops/actionengine"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

type Worker struct {
	engine opsaction.Engine
}

func NewWorker(engine opsaction.Engine) *Worker {
	return &Worker{engine: engine}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.mitigation.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	if event.Envelope.EventType != "recommended_actions.created" {
		return nil
	}
	if w.engine == nil {
		return nil
	}
	incidentID := worker.ResolveIncidentID(event)
	actions := worker.ToAnySlice(event.Envelope.Payload["actions"])
	for _, actionAny := range actions {
		actionMap, ok := actionAny.(map[string]any)
		if !ok {
			continue
		}
		req := opsaction.EngineRequest{
			IncidentID:   incidentID,
			ActionType:   strings.TrimSpace(worker.AsString(actionMap["action_type"])),
			Scope:        worker.ToMap(actionMap["scope"]),
			TTLSeconds:   worker.AsInt(actionMap["ttl_seconds"], 600),
			Params:       worker.ToMap(actionMap["params"]),
			EvidenceRefs: worker.ToStringSlice(actionMap["evidence_refs"]),
		}
		dryRun, err := w.engine.DryRun(ctx, event.Envelope.OrgID, worker.Ptr("agents.mitigation"), req)
		if err != nil {
			return err
		}
		if dryRun.ApprovalRequired {
			continue
		}
		proposalID := dryRun.Proposal.ID
		if _, err := w.engine.Apply(ctx, event.Envelope.OrgID, worker.Ptr("agents.mitigation"), opsaction.EngineRequest{
			ProposalID: &proposalID,
		}); err != nil {
			return err
		}
	}
	return nil
}

