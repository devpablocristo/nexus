package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nexus-core/pkg/validations/jsonschema"
)

func TestMockClient_GenerateStrict(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	client := NewClient(Config{
		Provider:  "mock",
		Model:     "mock-default",
		SchemaDir: filepath.Join(root, "internal", "ops", "schemas", "llm"),
	}, jsonschema.NewCompilerCache())

	if _, err := client.GenerateStrict(context.Background(), Request{
		Task:  "diagnosis",
		Input: map[string]any{"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54"},
	}, "diagnosis_report.json"); err != nil {
		t.Fatalf("diagnosis strict generation failed: %v", err)
	}

	if _, err := client.GenerateStrict(context.Background(), Request{
		Task:  "communication_plan",
		Input: map[string]any{"incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f"},
	}, "communication_plan.json"); err != nil {
		t.Fatalf("communication strict generation failed: %v", err)
	}

	if _, err := client.GenerateStrict(context.Background(), Request{
		Task:  "executive_qa",
		Input: map[string]any{"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54", "question": "status?"},
	}, "executive_qa_response.json"); err != nil {
		t.Fatalf("executive qa strict generation failed: %v", err)
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
