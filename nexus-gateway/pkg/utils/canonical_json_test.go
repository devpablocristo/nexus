package utils

import "testing"

func TestCanonicalJSONStable(t *testing.T) {
	v1 := map[string]any{"b": 2, "a": map[string]any{"y": 2, "x": 1}}
	v2 := map[string]any{"a": map[string]any{"x": 1, "y": 2}, "b": 2}

	b1, err := CanonicalJSON(v1)
	if err != nil {
		t.Fatalf("canonical json v1: %v", err)
	}
	b2, err := CanonicalJSON(v2)
	if err != nil {
		t.Fatalf("canonical json v2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("expected canonical output equality, got %s vs %s", string(b1), string(b2))
	}
}
