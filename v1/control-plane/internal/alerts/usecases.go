package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domain "control-plane/internal/alerts/usecases/domain"
	saasmetrics "control-plane/internal/shared/metrics"
)

type RepoPort interface {
	Create(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.AlertRule, error)
	ListEnabled(ctx context.Context) ([]domain.AlertRule, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	MarkFired(ctx context.Context, id uuid.UUID) error
}

// MetricsSource provides current metric values for alert evaluation.
type MetricsSource interface {
	DenyRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error)
	ErrorRate(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error)
	RateLimitedCount(ctx context.Context, orgID uuid.UUID, toolName *string, windowSeconds int) (float64, error)
}

type Usecases struct {
	repo    RepoPort
	metrics MetricsSource
	log     zerolog.Logger
	client  *http.Client
}

func NewUsecases(repo RepoPort, metrics MetricsSource, log zerolog.Logger) *Usecases {
	return &Usecases{
		repo:    repo,
		metrics: metrics,
		log:     log,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (u *Usecases) Create(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	return u.repo.Create(ctx, rule)
}

func (u *Usecases) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.AlertRule, error) {
	return u.repo.ListByOrg(ctx, orgID)
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return u.repo.Delete(ctx, orgID, id)
}

func (u *Usecases) EvaluateAll(ctx context.Context) (int, error) {
	saasmetrics.AlertsEvaluated.Inc()
	rules, err := u.repo.ListEnabled(ctx)
	if err != nil {
		return 0, err
	}
	fired := 0
	for _, rule := range rules {
		if rule.LastFiredAt != nil && time.Since(*rule.LastFiredAt) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}
		value, err := u.getCurrentValue(ctx, rule)
		if err != nil {
			u.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("alert_metric_fetch_failed")
			continue
		}
		if value < rule.Threshold {
			continue
		}
		payload := domain.WebhookPayload{
			AlertRuleID:   rule.ID.String(),
			AlertName:     rule.Name,
			Metric:        string(rule.Metric),
			Threshold:     rule.Threshold,
			CurrentValue:  value,
			WindowSeconds: rule.WindowSeconds,
			FiredAt:       time.Now().UTC().Format(time.RFC3339),
		}
		if rule.ToolName != nil {
			payload.ToolName = *rule.ToolName
		}
		if err := u.fireWebhook(ctx, rule.WebhookURL, payload); err != nil {
			u.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("webhook_fire_failed")
			continue
		}
		if err := u.repo.MarkFired(ctx, rule.ID); err != nil {
			u.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("mark_fired_failed")
		}
		label := strings.TrimSpace(rule.Name)
		if label == "" {
			label = rule.ID.String()
		}
		saasmetrics.AlertsFired.WithLabelValues(label).Inc()
		fired++
	}
	return fired, nil
}

func (u *Usecases) getCurrentValue(ctx context.Context, rule domain.AlertRule) (float64, error) {
	if u.metrics == nil {
		return 0, nil
	}
	switch rule.Metric {
	case domain.MetricDenyRate:
		return u.metrics.DenyRate(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	case domain.MetricErrorRate:
		return u.metrics.ErrorRate(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	case domain.MetricRateLimitCount:
		return u.metrics.RateLimitedCount(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	default:
		return 0, nil
	}
}

func (u *Usecases) fireWebhook(_ context.Context, url string, payload domain.WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := u.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
