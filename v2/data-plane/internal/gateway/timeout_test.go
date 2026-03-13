package gateway

import "testing"

func TestClampTimeoutMS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested int
		want      int
	}{
		{
			name: "uses default when missing",
			want: defaultTimeoutMS,
		},
		{
			name:      "clamps to min",
			requested: 1,
			want:      minTimeoutMS,
		},
		{
			name:      "clamps to max",
			requested: 60000,
			want:      maxTimeoutMS,
		},
		{
			name:      "keeps valid value",
			requested: 2500,
			want:      2500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := clampTimeoutMS(tt.requested)
			if got != tt.want {
				t.Fatalf("unexpected timeout: got=%d want=%d", got, tt.want)
			}
		})
	}
}
