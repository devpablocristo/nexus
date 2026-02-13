package policy

import "testing"

func TestEvaluator_BoolLogic_AllAnyNot(t *testing.T) {
	ev := NewEvaluator()
	cond := []byte(`{"all":[{"path":"input.amount","op":"lte","value":1000},{"not":{"path":"context.user_id","op":"not_exists"}}]}`)
	ok, err := ev.Matches(cond, map[string]any{"amount": 1000.0}, map[string]any{"user_id": "u1"}, ToolAttributes{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
}

func TestEvaluator_Ops(t *testing.T) {
	ev := NewEvaluator()
	tests := []struct {
		name string
		cond string
		in   map[string]any
		ctx  map[string]any
		want bool
	}{
		{"eq", `{"path":"input.a","op":"eq","value":1}`, map[string]any{"a": 1.0}, nil, true},
		{"neq", `{"path":"input.a","op":"neq","value":2}`, map[string]any{"a": 1.0}, nil, true},
		{"lt", `{"path":"input.a","op":"lt","value":2}`, map[string]any{"a": 1.0}, nil, true},
		{"lte", `{"path":"input.a","op":"lte","value":1}`, map[string]any{"a": 1.0}, nil, true},
		{"gt", `{"path":"input.a","op":"gt","value":0}`, map[string]any{"a": 1.0}, nil, true},
		{"gte", `{"path":"input.a","op":"gte","value":1}`, map[string]any{"a": 1.0}, nil, true},
		{"in", `{"path":"input.a","op":"in","value":[1,2,3]}`, map[string]any{"a": 2.0}, nil, true},
		{"contains_string", `{"path":"input.s","op":"contains","value":"bc"}`, map[string]any{"s": "abcd"}, nil, true},
		{"contains_array", `{"path":"input.arr","op":"contains","value":"x"}`, map[string]any{"arr": []any{"x", "y"}}, nil, true},
		{"regex", `{"path":"input.s","op":"regex","value":"^a.+d$"}`, map[string]any{"s": "abcd"}, nil, true},
		{"exists", `{"path":"context.user_id","op":"exists"}`, nil, map[string]any{"user_id": "u"}, true},
		{"not_exists", `{"path":"context.user_id","op":"not_exists"}`, nil, map[string]any{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := ev.Matches([]byte(tt.cond), tt.in, tt.ctx, ToolAttributes{})
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if ok != tt.want {
				t.Fatalf("want %v got %v", tt.want, ok)
			}
		})
	}
}

func TestEvaluator_ToolNamespace(t *testing.T) {
	ev := NewEvaluator()
	cond := []byte(`{"all":[{"path":"tool.action_type","op":"eq","value":"write"},{"path":"tool.risk_level","op":"gte","value":3}]}`)
	ok, err := ev.Matches(cond, map[string]any{}, map[string]any{}, ToolAttributes{
		ActionType: "write",
		RiskLevel:  3,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
}

func TestEvaluator_ExistsNull(t *testing.T) {
	ev := NewEvaluator()
	ok, err := ev.Matches([]byte(`{"path":"context.user_id","op":"exists"}`), nil, map[string]any{"user_id": nil}, ToolAttributes{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected not match when null")
	}
}

func TestEvaluator_RegexInvalidPattern(t *testing.T) {
	ev := NewEvaluator()
	_, err := ev.Matches([]byte(`{"path":"input.s","op":"regex","value":"["}`), map[string]any{"s": "x"}, nil, ToolAttributes{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
