package risk

import (
	"encoding/json"
	"testing"
	"time"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

func TestEvaluatorEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		input              Input
		wantLevel          actiondomain.RiskLevel
		wantScore          int
		wantDecision       actiondomain.RiskDecision
		wantRiskPressure   float64
		wantSafetyPressure float64
		wantActiveFactors  int
	}{
		{
			name: "withdrawal cold start recommends enhanced log",
			input: Input{
				ActionType: actiondomain.ActionTypeWithdrawal,
				Resource: actiondomain.ProtectedResource{
					ID:          "wallet_hot_usdc_1",
					Type:        actiondomain.ResourceTypeWallet,
					Criticality: "medium",
				},
				Payload: mustRawJSON(t, map[string]any{
					"asset":               "USDC",
					"amount":              "25000.00",
					"network":             "ethereum",
					"destination_address": "0xabc",
				}),
			},
			wantLevel:          actiondomain.RiskLevelMedium,
			wantScore:          20,
			wantDecision:       actiondomain.RiskDecisionEnhancedLog,
			wantRiskPressure:   0.20,
			wantSafetyPressure: 0,
			wantActiveFactors:  2,
		},
		{
			name: "internal hot to cold move stays low in first slice",
			input: Input{
				ActionType: actiondomain.ActionTypeHotToColdMove,
				Resource: actiondomain.ProtectedResource{
					ID:          "wallet_hot_btc_1",
					Type:        actiondomain.ResourceTypeWallet,
					Criticality: "medium",
				},
				Payload: mustRawJSON(t, map[string]any{
					"asset":       "BTC",
					"amount":      "1.50",
					"network":     "bitcoin",
					"from_wallet": "wallet_hot_btc_1",
					"to_wallet":   "wallet_cold_btc_1",
				}),
			},
			wantLevel:          actiondomain.RiskLevelLow,
			wantScore:          10,
			wantDecision:       actiondomain.RiskDecisionAllow,
			wantRiskPressure:   0.10,
			wantSafetyPressure: 0,
			wantActiveFactors:  2,
		},
		{
			name: "critical resource keeps full cold-start amount weight",
			input: Input{
				ActionType: actiondomain.ActionTypeWithdrawal,
				Resource: actiondomain.ProtectedResource{
					ID:          "wallet_hot_btc_critical",
					Type:        actiondomain.ResourceTypeWallet,
					Criticality: "critical",
				},
				Payload: mustRawJSON(t, map[string]any{
					"asset":               "BTC",
					"amount":              "10.00",
					"network":             "bitcoin",
					"destination_address": "bc1qxyz",
				}),
			},
			wantLevel:          actiondomain.RiskLevelMedium,
			wantScore:          30,
			wantDecision:       actiondomain.RiskDecisionEnhancedLog,
			wantRiskPressure:   0.30,
			wantSafetyPressure: 0,
			wantActiveFactors:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := (Evaluator{}).Evaluate(tt.input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got.Level != tt.wantLevel {
				t.Fatalf("Evaluate() level = %s, want %s", got.Level, tt.wantLevel)
			}
			if got.Score != tt.wantScore {
				t.Fatalf("Evaluate() score = %d, want %d", got.Score, tt.wantScore)
			}
			if got.RecommendedDecision != tt.wantDecision {
				t.Fatalf("Evaluate() recommended decision = %s, want %s", got.RecommendedDecision, tt.wantDecision)
			}
			if got.RiskPressure != tt.wantRiskPressure {
				t.Fatalf("Evaluate() risk pressure = %.2f, want %.2f", got.RiskPressure, tt.wantRiskPressure)
			}
			if got.SafetyPressure != tt.wantSafetyPressure {
				t.Fatalf("Evaluate() safety pressure = %.2f, want %.2f", got.SafetyPressure, tt.wantSafetyPressure)
			}
			if got.Profile.Name != "balanced" || got.Profile.Version != 1 {
				t.Fatalf("Evaluate() profile = %#v, want balanced v1", got.Profile)
			}

			active := 0
			for _, factor := range got.Factors {
				if factor.Active {
					active++
				}
			}
			if active != tt.wantActiveFactors {
				t.Fatalf("Evaluate() active factor count = %d, want %d", active, tt.wantActiveFactors)
			}
		})
	}
}

func TestEvaluatorEvaluateAmplifiesSuspiciousCombination(t *testing.T) {
	t.Parallel()

	ctx := Context{
		ResourceBaselines: map[Metric]Baseline{
			MetricAvgTxAmount: {
				ScopeType:  ScopeTypeResource,
				ScopeID:    "wallet_hot_usdc_1",
				Metric:     MetricAvgTxAmount,
				Avg:        100,
				Stddev:     10,
				P95:        120,
				SampleSize: 30,
				WindowDays: 30,
				ComputedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		ActorBaselines: map[Metric]Baseline{
			MetricActions30mCount: {
				ScopeType:  ScopeTypeActor,
				ScopeID:    "treasury-agent",
				Metric:     MetricActions30mCount,
				Avg:        1,
				Stddev:     0.2,
				P95:        1,
				SampleSize: 20,
				WindowDays: 30,
				ComputedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			},
			MetricTypicalHours: {
				ScopeType:  ScopeTypeActor,
				ScopeID:    "treasury-agent",
				Metric:     MetricTypicalHours,
				Avg:        10,
				Stddev:     1,
				P95:        11,
				SampleSize: 20,
				WindowDays: 30,
				ComputedAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		RecentActorCount30: 5,
		OpenIncidentCount:  2,
	}

	got, err := (Evaluator{}).Evaluate(Input{
		ActionType: actiondomain.ActionTypeWithdrawal,
		Resource: actiondomain.ProtectedResource{
			ID:          "wallet_hot_usdc_1",
			Type:        actiondomain.ResourceTypeWallet,
			Criticality: "high",
		},
		Actor: actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "treasury-agent"},
		Payload: mustRawJSON(t, map[string]any{
			"asset":               "USDC",
			"amount":              "1000.00",
			"network":             "ethereum",
			"destination_address": "0xnewdest",
		}),
		Now:     time.Date(2026, 3, 16, 3, 0, 0, 0, time.UTC),
		Context: ctx,
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got.RecommendedDecision != actiondomain.RiskDecisionDeny {
		t.Fatalf("Evaluate() recommended decision = %s, want %s", got.RecommendedDecision, actiondomain.RiskDecisionDeny)
	}
	if got.Score != 100 {
		t.Fatalf("Evaluate() score = %d, want 100", got.Score)
	}
	if len(got.Amplifications) != 3 {
		t.Fatalf("Evaluate() amplifications = %#v, want 3 active combinations", got.Amplifications)
	}
}

func TestApplyHysteresisKeepsPreviousDecisionNearBoundary(t *testing.T) {
	t.Parallel()

	previous := actiondomain.RiskDecisionRequireApproval
	got := applyHysteresis(
		actiondomain.RiskDecisionAdditionalAuth,
		&previous,
		[4]float64{0.20, 0.40, 0.60, 0.80},
		0.59,
	)
	if got != actiondomain.RiskDecisionRequireApproval {
		t.Fatalf("applyHysteresis() = %s, want %s", got, actiondomain.RiskDecisionRequireApproval)
	}
}

func mustRawJSON(t *testing.T, payload map[string]any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return raw
}
