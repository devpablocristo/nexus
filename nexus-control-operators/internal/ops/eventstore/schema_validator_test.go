package eventstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
	"nexus/pkg/validations/jsonschema"
)

func TestSchemaValidator_ValidEnvelopeAndPayload(t *testing.T) {
	t.Parallel()

	v := NewSchemaValidator(jsonschema.NewCompilerCache(), filepath.Join(repoRoot(t), "internal", "ops", "schemas", "events"))
	event := opsdomain.Envelope{
		ID:         uuid.New(),
		EventType:  "anomaly.detected",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		OrgID:      uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54"),
		Correlation: opsdomain.Correlation{
			RequestID: ptr("req-1"),
		},
		Actor: opsdomain.Actor{
			ActorID:   ptr("sentry"),
			ActorType: "agent",
		},
		Source: "agents.sentry",
		Payload: map[string]any{
			"fingerprint":     "fp:org:tool:error_rate",
			"signal":          "error_rate_spike",
			"tool_name":       "echo",
			"observed_value":  0.8,
			"threshold_value": 0.3,
		},
	}

	if err := v.ValidateEnvelope(context.Background(), event); err != nil {
		t.Fatalf("envelope should be valid: %v", err)
	}
	if err := v.ValidatePayload(context.Background(), event.EventType, event.Version, event.Payload); err != nil {
		t.Fatalf("payload should be valid: %v", err)
	}
}

func TestSchemaValidator_InvalidPayload(t *testing.T) {
	t.Parallel()

	v := NewSchemaValidator(jsonschema.NewCompilerCache(), filepath.Join(repoRoot(t), "internal", "ops", "schemas", "events"))
	payload := map[string]any{
		"incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f",
		"severity":    "HIGH",
	}
	if err := v.ValidatePayload(context.Background(), "incident.opened", 1, payload); err == nil {
		t.Fatalf("expected invalid payload to fail")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	cur := wd
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		next := filepath.Dir(cur)
		if next == cur {
			t.Fatalf("go.mod not found from %s", wd)
		}
		cur = next
	}
}

func ptr(s string) *string {
	return &s
}
