package action

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	actionrisk "nexus/v2/data-plane/internal/action/risk"
	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

func TestHistoricalRiskContextProviderRefreshAllBuildsBaselines(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 15, 0, 0, 0, time.UTC)
	repo := NewInMemoryRepository([]actiondomain.Action{
		testHistoricalAction(t, uuid.New(), now.Add(-48*time.Hour), "wallet_hot_usdc_1", "treasury-agent", "0xabc", "100.00"),
		testHistoricalAction(t, uuid.New(), now.Add(-24*time.Hour), "wallet_hot_usdc_1", "treasury-agent", "0xabc", "120.00"),
		testHistoricalAction(t, uuid.New(), now.Add(-20*time.Minute), "wallet_hot_usdc_1", "treasury-agent", "0xabc", "110.00"),
	})
	store := NewInMemoryRiskBaselineStore()
	provider := NewHistoricalRiskContextProvider(repo, store)

	if err := provider.RefreshAll(context.Background()); err != nil {
		t.Fatalf("RefreshAll() error = %v", err)
	}

	ctx, err := provider.ContextFor(context.Background(), CreateRequest{
		ActionType:   actiondomain.ActionTypeWithdrawal,
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: actiondomain.ResourceTypeWallet,
		ProposedBy:   actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "treasury-agent"},
		Payload:      mustActionPayload(t, "0xabc", "115.00"),
	}, actiondomain.ProtectedResource{
		ID:          "wallet_hot_usdc_1",
		Type:        actiondomain.ResourceTypeWallet,
		Criticality: "medium",
	}, now)
	if err != nil {
		t.Fatalf("ContextFor() error = %v", err)
	}

	amountBaseline, ok := ctx.ResourceBaselines[actionrisk.MetricAvgTxAmount]
	if !ok || amountBaseline.SampleSize != 2 {
		t.Fatalf("ContextFor() resource baseline missing or wrong: %#v", ctx.ResourceBaselines)
	}
	if amountBaseline.Confidence() <= 0 {
		t.Fatalf("baseline confidence = %.2f, want > 0", amountBaseline.Confidence())
	}
	if ctx.KnownDestination == nil || ctx.KnownDestination.Destination != "0xabc" {
		t.Fatalf("ContextFor() known destination = %#v, want 0xabc", ctx.KnownDestination)
	}
	if ctx.RecentActorCount30 != 2 {
		t.Fatalf("ContextFor() recent actor count = %d, want 2", ctx.RecentActorCount30)
	}
	if ctx.PreviousDecision == nil || *ctx.PreviousDecision != actiondomain.RiskDecisionEnhancedLog {
		t.Fatalf("ContextFor() previous decision = %#v, want enhanced_log", ctx.PreviousDecision)
	}
}

func TestBaselineConfidenceIsSaturating(t *testing.T) {
	t.Parallel()

	confidence3 := actionrisk.Baseline{SampleSize: 3}.Confidence()
	confidence7 := actionrisk.Baseline{SampleSize: 7}.Confidence()
	confidence30 := actionrisk.Baseline{SampleSize: 30}.Confidence()
	if !(confidence3 < confidence7 && confidence7 < confidence30) {
		t.Fatalf("confidence should be monotonic, got 3=%.2f 7=%.2f 30=%.2f", confidence3, confidence7, confidence30)
	}
	if confidence30 >= 1 {
		t.Fatalf("confidence30 = %.2f, want < 1", confidence30)
	}
}

func testHistoricalAction(t *testing.T, id uuid.UUID, createdAt time.Time, resourceID, actorID, destination, amount string) actiondomain.Action {
	t.Helper()

	payload, err := json.Marshal(actiondomain.WithdrawalPayload{
		Asset:              "USDC",
		Amount:             amount,
		Network:            "ethereum",
		DestinationAddress: destination,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return actiondomain.Action{
		ID:           id,
		Type:         actiondomain.ActionTypeWithdrawal,
		Status:       actiondomain.ActionStatusPendingApproval,
		Decision:     actiondomain.DecisionRequireApproval,
		ResourceID:   resourceID,
		ResourceType: actiondomain.ResourceTypeWallet,
		RequestedBy:  actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "treasury-bot"},
		ProposedBy:   actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: actorID},
		Payload:      payload,
		Risk: actiondomain.RiskAssessment{
			RecommendedDecision: actiondomain.RiskDecisionEnhancedLog,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func mustActionPayload(t *testing.T, destination, amount string) json.RawMessage {
	t.Helper()

	payload, err := json.Marshal(actiondomain.WithdrawalPayload{
		Asset:              "USDC",
		Amount:             amount,
		Network:            "ethereum",
		DestinationAddress: destination,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}
