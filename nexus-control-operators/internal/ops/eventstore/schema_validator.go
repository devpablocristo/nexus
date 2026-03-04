package eventstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	js "github.com/santhosh-tekuri/jsonschema/v5"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
	"nexus-control-operators/pkg/validations/jsonschema"
)

var eventTypeSchemaFilesV1 = map[string]string{
	"tool_call.finished":         "tool_call_finished_v1.json",
	"policy.denied":              "policy_denied_v1.json",
	"quota.exceeded":             "quota_exceeded_v1.json",
	"tool_degraded":              "tool_degraded_v1.json",
	"anomaly.detected":           "anomaly_detected_v1.json",
	"incident.opened":            "incident_opened_v1.json",
	"incident.state_changed":     "incident_state_changed_v1.json",
	"diagnosis.created":          "diagnosis_created_v1.json",
	"recommended_actions.created": "recommended_actions_created_v1.json",
	"action.proposed":            "action_proposed_v1.json",
	"action.dry_run_ok":          "action_dry_run_ok_v1.json",
	"action.dry_run_failed":      "action_dry_run_failed_v1.json",
	"action.applied":             "action_applied_v1.json",
	"action.failed":              "action_failed_v1.json",
	"action.rolled_back":         "action_rolled_back_v1.json",
	"comms.draft_created":        "comms_draft_created_v1.json",
	"comms.awaiting_approval":    "comms_awaiting_approval_v1.json",
	"comms.sent_internal":        "comms_sent_internal_v1.json",
}

type schemaValidator struct {
	cache      *jsonschema.CompilerCache
	schemaDir  string
}

func NewSchemaValidator(cache *jsonschema.CompilerCache, schemaDir string) ValidatorPort {
	dir := schemaDir
	if dir == "" {
		dir = filepath.Join("internal", "ops", "schemas", "events")
	}
	return &schemaValidator{
		cache:     cache,
		schemaDir: dir,
	}
}

func (v *schemaValidator) ValidateEnvelope(ctx context.Context, event opsdomain.Envelope) error {
	sch, err := v.compileFromFile(ctx, "ops-envelope-v1", filepath.Join(v.schemaDir, "envelope_v1.json"))
	if err != nil {
		return fmt.Errorf("compile envelope schema: %w", err)
	}
	envelopeObj := map[string]any{
		"id":          event.ID.String(),
		"event_type":  event.EventType,
		"version":     event.Version,
		"occurred_at": event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"org_id":      event.OrgID.String(),
		"correlation": map[string]any{},
		"actor": map[string]any{
			"actor_type": event.Actor.ActorType,
		},
		"source":  event.Source,
		"payload": event.Payload,
	}
	if event.Correlation.RequestID != nil && *event.Correlation.RequestID != "" {
		envelopeObj["correlation"].(map[string]any)["request_id"] = *event.Correlation.RequestID
	}
	if event.Correlation.IncidentID != nil && *event.Correlation.IncidentID != "" {
		envelopeObj["correlation"].(map[string]any)["incident_id"] = *event.Correlation.IncidentID
	}
	if event.Correlation.ActionID != nil && *event.Correlation.ActionID != "" {
		envelopeObj["correlation"].(map[string]any)["action_id"] = *event.Correlation.ActionID
	}
	if event.Actor.ActorID != nil && *event.Actor.ActorID != "" {
		envelopeObj["actor"].(map[string]any)["actor_id"] = *event.Actor.ActorID
	}
	return jsonschema.Validate(sch, envelopeObj)
}

func (v *schemaValidator) ValidatePayload(ctx context.Context, eventType string, version int, payload map[string]any) error {
	if version != 1 {
		return fmt.Errorf("unsupported event version: %d", version)
	}
	fileName, ok := eventTypeSchemaFilesV1[eventType]
	if !ok {
		return fmt.Errorf("unsupported event_type: %s", eventType)
	}
	sch, err := v.compileFromFile(ctx, "ops-payload-"+eventType+"-v1", filepath.Join(v.schemaDir, fileName))
	if err != nil {
		return fmt.Errorf("compile payload schema: %w", err)
	}
	return jsonschema.Validate(sch, payload)
}

func (v *schemaValidator) compileFromFile(ctx context.Context, cacheKey, path string) (*js.Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return v.cache.Compile(ctx, cacheKey, raw)
}
