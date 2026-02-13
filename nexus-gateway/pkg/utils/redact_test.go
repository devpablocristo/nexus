package utils

import "testing"

func TestRedact_NestedMixedCase(t *testing.T) {
	in := map[string]any{
		"Token": "secret",
		"nested": map[string]any{
			"password": "p",
			"arr": []any{
				map[string]any{"ApiKey": "k"},
				"ok",
			},
		},
	}

	out, ok := Redact(in).(map[string]any)
	if !ok {
		t.Fatalf("expected map")
	}

	if out["Token"] != "***" {
		t.Fatalf("expected Token redacted")
	}
	nested := out["nested"].(map[string]any)
	if nested["password"] != "***" {
		t.Fatalf("expected password redacted")
	}
	arr := nested["arr"].([]any)
	m := arr[0].(map[string]any)
	if m["ApiKey"] != "***" {
		t.Fatalf("expected ApiKey redacted")
	}
	if arr[1].(string) != "ok" {
		t.Fatalf("expected non-sensitive preserved")
	}
}
