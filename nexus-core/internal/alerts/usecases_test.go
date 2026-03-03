package alerts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domain "nexus-core/internal/alerts/usecases/domain"
)

type stubAlertRepo struct {
	rules     []domain.AlertRule
	firedID   uuid.UUID
	deletedID uuid.UUID
}

func (r *stubAlertRepo) Create(_ context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	r.rules = append(r.rules, rule)
	return rule, nil
}

func (r *stubAlertRepo) ListByOrg(_ context.Context, orgID uuid.UUID) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, rule := range r.rules {
		if rule.OrgID == orgID {
			out = append(out, rule)
		}
	}
	return out, nil
}

func (r *stubAlertRepo) ListEnabled(_ context.Context) ([]domain.AlertRule, error) {
	var out []domain.AlertRule
	for _, rule := range r.rules {
		if rule.Enabled {
			out = append(out, rule)
		}
	}
	return out, nil
}

func (r *stubAlertRepo) Delete(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	r.deletedID = id
	return nil
}

func (r *stubAlertRepo) MarkFired(_ context.Context, id uuid.UUID) error {
	r.firedID = id
	return nil
}

type stubMetrics struct {
	denyRate float64
}

func (m *stubMetrics) DenyRate(_ context.Context, _ uuid.UUID, _ *string, _ int) (float64, error) {
	return m.denyRate, nil
}
func (m *stubMetrics) ErrorRate(_ context.Context, _ uuid.UUID, _ *string, _ int) (float64, error) {
	return 0, nil
}
func (m *stubMetrics) RateLimitedCount(_ context.Context, _ uuid.UUID, _ *string, _ int) (float64, error) {
	return 0, nil
}

func TestCreateAndList(t *testing.T) {
	repo := &stubAlertRepo{}
	log := zerolog.New(io.Discard)
	svc := NewUsecases(repo, nil, log)

	orgID := uuid.New()
	rule, err := svc.Create(context.Background(), domain.AlertRule{
		OrgID:     orgID,
		Name:      "high-deny",
		Metric:    domain.MetricDenyRate,
		Threshold: 0.5,
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Name != "high-deny" {
		t.Error("name mismatch")
	}

	rules, err := svc.ListByOrg(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1, got %d", len(rules))
	}
}

func TestDelete(t *testing.T) {
	repo := &stubAlertRepo{}
	log := zerolog.New(io.Discard)
	svc := NewUsecases(repo, nil, log)

	id := uuid.New()
	if err := svc.Delete(context.Background(), uuid.New(), id); err != nil {
		t.Fatal(err)
	}
	if repo.deletedID != id {
		t.Error("delete not called")
	}
}

func TestEvaluateAll_FiresWebhook(t *testing.T) {
	var received bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(200)
	}))
	defer ts.Close()

	orgID := uuid.New()
	repo := &stubAlertRepo{
		rules: []domain.AlertRule{{
			ID:              uuid.New(),
			OrgID:           orgID,
			Name:            "test-alert",
			Metric:          domain.MetricDenyRate,
			Threshold:       0.1,
			WindowSeconds:   300,
			WebhookURL:      ts.URL,
			CooldownSeconds: 60,
			Enabled:         true,
		}},
	}
	metrics := &stubMetrics{denyRate: 0.8}
	log := zerolog.New(io.Discard)
	svc := NewUsecases(repo, metrics, log)

	fired, err := svc.EvaluateAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fired != 1 {
		t.Errorf("expected 1 fired, got %d", fired)
	}
	if !received {
		t.Error("webhook not called")
	}
	if repo.firedID == uuid.Nil {
		t.Error("MarkFired not called")
	}
}

func TestEvaluateAll_NilMetrics(t *testing.T) {
	repo := &stubAlertRepo{
		rules: []domain.AlertRule{{
			ID:        uuid.New(),
			Metric:    domain.MetricDenyRate,
			Threshold: 0.1,
			Enabled:   true,
		}},
	}
	log := zerolog.New(io.Discard)
	svc := NewUsecases(repo, nil, log)

	fired, err := svc.EvaluateAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fired != 0 {
		t.Errorf("expected 0 (nil metrics returns 0), got %d", fired)
	}
}

func TestEvaluateAll_CooldownSkip(t *testing.T) {
	now := time.Now()
	repo := &stubAlertRepo{
		rules: []domain.AlertRule{{
			ID:              uuid.New(),
			Metric:          domain.MetricDenyRate,
			Threshold:       0.1,
			CooldownSeconds: 3600,
			Enabled:         true,
			LastFiredAt:     &now,
		}},
	}
	metrics := &stubMetrics{denyRate: 0.8}
	log := zerolog.New(io.Discard)
	svc := NewUsecases(repo, metrics, log)

	fired, err := svc.EvaluateAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fired != 0 {
		t.Errorf("expected 0 (cooldown), got %d", fired)
	}
}
