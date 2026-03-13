package policy

import "testing"

func TestEvaluatorMatches(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	tests := []struct {
		name       string
		expression string
		input      map[string]any
		context    map[string]any
		tool       ToolAttributes
		want       bool
		wantErr    bool
	}{
		{
			name:       "empty expression matches",
			expression: "",
			want:       true,
		},
		{
			name:       "matches input and tool",
			expression: `input.hello == "blocked" && tool.method == "POST"`,
			input:      map[string]any{"hello": "blocked"},
			tool:       ToolAttributes{Name: "echo", Kind: "http", Method: "POST"},
			want:       true,
		},
		{
			name:       "matches context",
			expression: `context.actor == "pablo"`,
			context:    map[string]any{"actor": "pablo"},
			want:       true,
		},
		{
			name:       "supports regex",
			expression: `input.hello.matches("^block")`,
			input:      map[string]any{"hello": "blocked"},
			want:       true,
		},
		{
			name:       "invalid expression returns error",
			expression: `input.hello ==`,
			wantErr:    true,
		},
		{
			name:       "non bool expression returns error",
			expression: `"blocked"`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := evaluator.Matches(tt.expression, tt.input, tt.context, tt.tool)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected result: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestEvaluatorValidate(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "accepts bool expression",
			expression: `input.hello == "blocked"`,
		},
		{
			name:       "rejects invalid syntax",
			expression: `input.hello ==`,
			wantErr:    true,
		},
		{
			name:       "rejects non bool expression",
			expression: `"blocked"`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := evaluator.Validate(tt.expression)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEvaluatorProgramCacheIsBounded(t *testing.T) {
	t.Parallel()

	evaluator := newEvaluatorWithLimit(2)

	for _, expression := range []string{
		`input.hello == "one"`,
		`input.hello == "two"`,
		`input.hello == "three"`,
	} {
		if err := evaluator.Validate(expression); err != nil {
			t.Fatalf("validate expression %q: %v", expression, err)
		}
	}

	evaluator.mu.Lock()
	defer evaluator.mu.Unlock()

	if got := len(evaluator.programs); got != 2 {
		t.Fatalf("unexpected cache size: got=%d want=2", got)
	}
	if _, ok := evaluator.programs[`input.hello == "one"`]; ok {
		t.Fatal("expected oldest expression to be evicted")
	}
}
