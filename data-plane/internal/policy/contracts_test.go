package policy

import "testing"

func TestPolicyDSLContract_InvalidJSONReturnsError(t *testing.T) {
	ev := NewEvaluator()

	_, err := ev.Matches([]byte(`{"all":[}`), map[string]any{}, map[string]any{}, ToolAttributes{})
	if err == nil {
		t.Fatalf("expected invalid json error")
	}
}

func TestPolicyDSLContract_UnknownNamespaceDoesNotMatch(t *testing.T) {
	ev := NewEvaluator()

	ok, err := ev.Matches(
		[]byte(`{"path":"tenant.plan_code","op":"eq","value":"growth"}`),
		map[string]any{},
		map[string]any{},
		ToolAttributes{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected unknown namespace to not match")
	}
}

func TestPolicyDSLContract_UnknownOperatorDoesNotMatch(t *testing.T) {
	ev := NewEvaluator()

	ok, err := ev.Matches(
		[]byte(`{"path":"input.amount","op":"starts_with","value":"1"}`),
		map[string]any{"amount": 10.0},
		map[string]any{},
		ToolAttributes{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected unknown operator to not match")
	}
}
