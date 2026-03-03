package schemas_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	js "github.com/santhosh-tekuri/jsonschema/v5"
	"nexus-core/pkg/validations/jsonschema"
)

func TestEventFixturesValidateAgainstEnvelopeAndEventSchema(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cache := jsonschema.NewCompilerCache()
	envelopeSchema := mustCompileSchema(t, cache, "event-envelope-v1", filepath.Join(root, "internal/ops/schemas/events/envelope_v1.json"))

	cases := []struct {
		fixture     string
		eventType   string
		eventSchema string
	}{
		{
			fixture:     filepath.Join(root, "testdata/events/tool_call_finished.valid.json"),
			eventType:   "tool_call.finished",
			eventSchema: filepath.Join(root, "internal/ops/schemas/events/tool_call_finished_v1.json"),
		},
		{
			fixture:     filepath.Join(root, "testdata/events/anomaly_detected.valid.json"),
			eventType:   "anomaly.detected",
			eventSchema: filepath.Join(root, "internal/ops/schemas/events/anomaly_detected_v1.json"),
		},
		{
			fixture:     filepath.Join(root, "testdata/events/action_applied.valid.json"),
			eventType:   "action.applied",
			eventSchema: filepath.Join(root, "internal/ops/schemas/events/action_applied_v1.json"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(filepath.Base(tc.fixture), func(t *testing.T) {
			t.Parallel()
			doc := mustReadJSONFile(t, tc.fixture)
			if err := jsonschema.Validate(envelopeSchema, doc); err != nil {
				t.Fatalf("envelope validation failed: %v", err)
			}
			if got := asString(doc["event_type"]); got != tc.eventType {
				t.Fatalf("unexpected event_type: got=%q want=%q", got, tc.eventType)
			}

			payload, ok := doc["payload"].(map[string]any)
			if !ok {
				t.Fatalf("payload is not object")
			}

			eventSchema := mustCompileSchema(t, cache, "event-"+tc.eventType, tc.eventSchema)
			if err := jsonschema.Validate(eventSchema, payload); err != nil {
				t.Fatalf("event payload validation failed: %v", err)
			}
		})
	}
}

func TestLLMSchemasValidateFixtures(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cache := jsonschema.NewCompilerCache()
	diagnosisSchema := mustCompileSchema(t, cache, "llm-diagnosis", filepath.Join(root, "internal/ops/schemas/llm/diagnosis_report.json"))
	commsSchema := mustCompileSchema(t, cache, "llm-comms", filepath.Join(root, "internal/ops/schemas/llm/communication_plan.json"))
	execQASchema := mustCompileSchema(t, cache, "llm-exec-qa", filepath.Join(root, "internal/ops/schemas/llm/executive_qa_response.json"))

	for _, p := range []string{
		filepath.Join(root, "testdata/llm/diagnosis.valid.json"),
		filepath.Join(root, "testdata/llm/diagnosis.unknown.valid.json"),
	} {
		p := p
		t.Run("valid-"+filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			doc := mustReadJSONFile(t, p)
			if err := jsonschema.Validate(diagnosisSchema, doc); err != nil {
				t.Fatalf("diagnosis schema should pass: %v", err)
			}
		})
	}

	t.Run("invalid-diagnosis-missing-evidence", func(t *testing.T) {
		doc := mustReadJSONFile(t, filepath.Join(root, "testdata/llm/diagnosis.invalid.json"))
		if err := jsonschema.Validate(diagnosisSchema, doc); err == nil {
			t.Fatalf("expected diagnosis invalid fixture to fail")
		}
	})

	t.Run("valid-communication-plan", func(t *testing.T) {
		doc := mustReadJSONFile(t, filepath.Join(root, "testdata/llm/communication_plan.valid.json"))
		if err := jsonschema.Validate(commsSchema, doc); err != nil {
			t.Fatalf("communication schema should pass: %v", err)
		}
	})

	t.Run("invalid-communication-plan", func(t *testing.T) {
		doc := mustReadJSONFile(t, filepath.Join(root, "testdata/llm/communication_plan.invalid.json"))
		if err := jsonschema.Validate(commsSchema, doc); err == nil {
			t.Fatalf("expected communication invalid fixture to fail")
		}
	})

	t.Run("valid-executive-qa", func(t *testing.T) {
		doc := mustReadJSONFile(t, filepath.Join(root, "testdata/llm/executive_qa.valid.json"))
		if err := jsonschema.Validate(execQASchema, doc); err != nil {
			t.Fatalf("executive qa schema should pass: %v", err)
		}
	})

	t.Run("invalid-executive-qa", func(t *testing.T) {
		doc := mustReadJSONFile(t, filepath.Join(root, "testdata/llm/executive_qa.invalid.json"))
		if err := jsonschema.Validate(execQASchema, doc); err == nil {
			t.Fatalf("expected executive qa invalid fixture to fail")
		}
	})
}

func TestActionSchemasCompileAndValidateExamples(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cache := jsonschema.NewCompilerCache()

	cases := []struct {
		schema  string
		example map[string]any
	}{
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/set_safe_mode_v1.json"),
			example: map[string]any{
				"action_type": "set_safe_mode",
				"scope": map[string]any{
					"level":  "org",
					"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"enabled": true,
				},
			},
		},
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/pause_tool_v1.json"),
			example: map[string]any{
				"action_type": "pause_tool",
				"scope": map[string]any{
					"level":   "tool",
					"org_id":  "996e9e43-7bab-4e68-a831-0a766befbf54",
					"tool_id": "echo",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"tool_id": "echo",
				},
			},
		},
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/quarantine_tenant_v1.json"),
			example: map[string]any{
				"action_type": "quarantine_tenant",
				"scope": map[string]any{
					"level":  "org",
					"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
					"mode":   "soft",
				},
			},
		},
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/set_rate_limit_v1.json"),
			example: map[string]any{
				"action_type": "set_rate_limit",
				"scope": map[string]any{
					"level":   "tool",
					"org_id":  "996e9e43-7bab-4e68-a831-0a766befbf54",
					"tool_id": "echo",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"rpm":     150,
					"tool_id": "echo",
				},
			},
		},
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/rollback_last_mitigation_v1.json"),
			example: map[string]any{
				"action_type": "rollback_last_mitigation",
				"scope": map[string]any{
					"level":  "org",
					"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f",
				},
			},
		},
		{
			schema: filepath.Join(root, "internal/ops/schemas/actions/export_audit_v1.json"),
			example: map[string]any{
				"action_type": "export_audit",
				"scope": map[string]any{
					"level":  "org",
					"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
				},
				"ttl_seconds": 300,
				"params": map[string]any{
					"format": "jsonl",
					"filters": map[string]any{
						"from": "2026-02-23T00:00:00Z",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(filepath.Base(tc.schema), func(t *testing.T) {
			t.Parallel()
			sch := mustCompileSchema(t, cache, "action-"+filepath.Base(tc.schema), tc.schema)
			if err := jsonschema.Validate(sch, tc.example); err != nil {
				t.Fatalf("action schema should pass: %v", err)
			}
		})
	}
}

func mustCompileSchema(t *testing.T, cache *jsonschema.CompilerCache, key, path string) *js.Schema {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", path, err)
	}
	sch, err := cache.Compile(context.Background(), key, raw)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return sch
}

func mustReadJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal json %s: %v", path, err)
	}
	return out
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

func asString(v any) string {
	s, _ := v.(string)
	return s
}
