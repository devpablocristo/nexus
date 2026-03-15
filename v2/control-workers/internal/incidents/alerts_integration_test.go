package incidents

import (
	"context"
	"testing"

	"nexus/v2/control-workers/internal/alerts"
	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

func TestCreateIncidentOpensAlertForHighSeverity(t *testing.T) {
	t.Parallel()

	alertRepo := alerts.NewInMemoryRepository(nil)
	alertUC := alerts.NewUsecases(alertRepo)
	uc := NewUsecases(NewInMemoryRepository(nil)).WithAlertSink(alertUC)

	created, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelHigh,
		Reason:       "blocked by policy",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	alertsList, err := alertUC.List(context.Background(), alerts.ListRequest{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(alertsList) != 1 {
		t.Fatalf("expected one alert, got %d", len(alertsList))
	}
	if alertsList[0].SourceID != created.ID || alertsList[0].Channel != alertdomain.ChannelSlack {
		t.Fatalf("unexpected alert: %#v", alertsList[0])
	}
}

func TestCreateIncidentSkipsAlertForMediumSeverity(t *testing.T) {
	t.Parallel()

	alertRepo := alerts.NewInMemoryRepository(nil)
	alertUC := alerts.NewUsecases(alertRepo)
	uc := NewUsecases(NewInMemoryRepository(nil)).WithAlertSink(alertUC)

	_, err := uc.Create(context.Background(), CreateRequest{
		SourceKind:   incidentdomain.SourceKindAction,
		SourceID:     "action-1",
		ActionType:   "withdrawal",
		ResourceID:   "wallet_hot_usdc_1",
		ResourceType: "wallet",
		Trigger:      incidentdomain.TriggerBlockedAction,
		RiskLevel:    incidentdomain.RiskLevelMedium,
		Reason:       "blocked by policy",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	alertsList, err := alertUC.List(context.Background(), alerts.ListRequest{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(alertsList) != 0 {
		t.Fatalf("expected no alerts, got %#v", alertsList)
	}
}
