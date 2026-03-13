package egress

import (
	"context"
	"testing"
)

func TestUsecasesIsHostAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rules  []Rule
		toolID string
		host   string
		want   bool
	}{
		{
			name:   "denies when tool has no rules",
			rules:  nil,
			toolID: "tool_echo",
			host:   "api.example.com",
			want:   false,
		},
		{
			name: "allows configured host case-insensitively",
			rules: []Rule{
				{ToolID: "tool_echo", Host: "api.example.com", Enabled: true},
			},
			toolID: "tool_echo",
			host:   "API.EXAMPLE.COM",
			want:   true,
		},
		{
			name: "denies different host",
			rules: []Rule{
				{ToolID: "tool_echo", Host: "api.example.com", Enabled: true},
			},
			toolID: "tool_echo",
			host:   "other.example.com",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewUsecases(NewInMemoryRepository(tt.rules))
			got, err := uc.IsHostAllowed(context.Background(), tt.toolID, tt.host)
			if err != nil {
				t.Fatalf("IsHostAllowed returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected result: got=%v want=%v", got, tt.want)
			}
		})
	}
}
