// Package mitigation auto-applies low-risk recommended actions.
package mitigation

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"control-workers/internal/agents/mitigation/worker"
	opsaction "control-workers/internal/ops/actionengine"
	opsdomain "control-workers/internal/ops/eventstore/usecases/domain"
)

type Worker struct {
	engine opsaction.Engine
	log    zerolog.Logger
}

func NewWorker(engine opsaction.Engine, log zerolog.Logger) *Worker {
	return &Worker{
		engine: engine,
		log:    log.With().Str("worker", "mitigation").Logger(),
	}
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
			LeaseHeaders: worker.ToStringMap(actionMap["lease_headers"]),
		}

		w.log.Info().
			Str("action_type", req.ActionType).
			Str("incident_id", fmt.Sprintf("%v", req.IncidentID)).
			Msg("executing dry-run")

		dryRun, err := w.engine.DryRun(ctx, event.Envelope.OrgID, worker.Ptr("agents.mitigation"), req)
		if err != nil {
			w.log.Error().Err(err).Str("action_type", req.ActionType).Msg("dry-run failed")
			return err
		}
		if dryRun.ApprovalRequired {
			w.log.Info().Str("action_type", req.ActionType).Msg("approval required, skipping auto-apply")
			continue
		}

		proposalID := dryRun.Proposal.ID
		w.log.Info().
			Str("proposal_id", proposalID.String()).
			Str("action_type", req.ActionType).
			Msg("applying action")

		if _, err := w.engine.Apply(ctx, event.Envelope.OrgID, worker.Ptr("agents.mitigation"), opsaction.EngineRequest{
			ProposalID:   &proposalID,
			LeaseHeaders: req.LeaseHeaders,
		}); err != nil {
			w.log.Error().Err(err).Str("proposal_id", proposalID.String()).Msg("apply failed")
			return err
		}
	}
	return nil
}
