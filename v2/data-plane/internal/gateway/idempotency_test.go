package gateway

import "testing"

func TestBuildRequestFingerprintStable(t *testing.T) {
	t.Parallel()

	first, err := buildRequestFingerprint(
		"echo",
		map[string]any{"b": 2, "a": 1},
		map[string]any{"z": true, "y": "x"},
	)
	if err != nil {
		t.Fatalf("first fingerprint: %v", err)
	}

	second, err := buildRequestFingerprint(
		"echo",
		map[string]any{"a": 1, "b": 2},
		map[string]any{"y": "x", "z": true},
	)
	if err != nil {
		t.Fatalf("second fingerprint: %v", err)
	}

	if first != second {
		t.Fatalf("expected stable fingerprint, got %s vs %s", first, second)
	}
}
