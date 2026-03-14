package gateway

import (
	"net/http"
	"testing"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
	"nexus/v2/data-plane/internal/tool"
)

func TestClassifyRiskClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tool      tool.Definition
		input     map[string]any
		context   map[string]any
		wantClass gwdomain.RiskClass
	}{
		{
			name:      "get is read",
			tool:      tool.Definition{Name: "echo", Method: http.MethodGet},
			wantClass: gwdomain.RiskClassRead,
		},
		{
			name:      "plan hint wins",
			tool:      tool.Definition{Name: "terraform", Method: http.MethodPost},
			input:     map[string]any{"plan_only": true},
			wantClass: gwdomain.RiskClassPlan,
		},
		{
			name:      "break glass hint wins",
			tool:      tool.Definition{Name: "kubectl", Method: http.MethodPost},
			context:   map[string]any{"approval_mode": "break_glass"},
			wantClass: gwdomain.RiskClassBreakGlass,
		},
		{
			name:      "delete prod is destructive prod",
			tool:      tool.Definition{Name: "kubectl", Method: http.MethodDelete},
			input:     map[string]any{"environment": "production"},
			wantClass: gwdomain.RiskClassDestructiveProd,
		},
		{
			name:      "post prod is mutate prod",
			tool:      tool.Definition{Name: "kubectl", Method: http.MethodPost},
			input:     map[string]any{"environment": "prod"},
			wantClass: gwdomain.RiskClassMutateProd,
		},
		{
			name:      "post non prod is mutate non prod",
			tool:      tool.Definition{Name: "echo", Method: http.MethodPost},
			input:     map[string]any{"environment": "staging"},
			wantClass: gwdomain.RiskClassMutateNonProd,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyRiskClass(tt.tool, tt.input, tt.context)
			if got != tt.wantClass {
				t.Fatalf("unexpected risk class: got=%s want=%s", got, tt.wantClass)
			}
		})
	}
}

func TestEvaluateDeterministicPreflight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		riskClass  gwdomain.RiskClass
		input      map[string]any
		context    map[string]any
		wantStatus gwdomain.PreflightStatus
		wantNeeded bool
	}{
		{
			name:       "read does not require preflight",
			riskClass:  gwdomain.RiskClassRead,
			wantStatus: gwdomain.PreflightStatusNotRequired,
		},
		{
			name:       "mutate prod passes with change ticket",
			riskClass:  gwdomain.RiskClassMutateProd,
			input:      map[string]any{"change_ticket": "CHG-1"},
			wantStatus: gwdomain.PreflightStatusPassed,
			wantNeeded: true,
		},
		{
			name:       "mutate prod fails without change ticket",
			riskClass:  gwdomain.RiskClassMutateProd,
			wantStatus: gwdomain.PreflightStatusFailed,
			wantNeeded: true,
		},
		{
			name:       "destructive prod passes with ticket and restore evidence",
			riskClass:  gwdomain.RiskClassDestructiveProd,
			input:      map[string]any{"change_ticket": "CHG-1", "restore_evidence": "snapshot-1"},
			wantStatus: gwdomain.PreflightStatusPassed,
			wantNeeded: true,
		},
		{
			name:       "break glass fails without incident and justification",
			riskClass:  gwdomain.RiskClassBreakGlass,
			wantStatus: gwdomain.PreflightStatusFailed,
			wantNeeded: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := evaluateDeterministicPreflight(tt.riskClass, tt.input, tt.context)
			if got.status != tt.wantStatus {
				t.Fatalf("unexpected preflight status: got=%s want=%s", got.status, tt.wantStatus)
			}
			if got.required != tt.wantNeeded {
				t.Fatalf("unexpected preflight required: got=%t want=%t", got.required, tt.wantNeeded)
			}
			if got.summary == nil {
				t.Fatalf("expected preflight summary")
			}
		})
	}
}
