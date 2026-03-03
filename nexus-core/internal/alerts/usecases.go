package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domain "nexus-core/internal/alerts/usecases/domain"
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

type Service struct {
	repo    RepoPort
	metrics MetricsSource
	log     zerolog.Logger
	client  *http.Client
}

func NewService(repo RepoPort, metrics MetricsSource, log zerolog.Logger) *Service {
	return &Service{
		repo:    repo,
		metrics: metrics,
		log:     log,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) Create(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	return s.repo.Create(ctx, rule)
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.AlertRule, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

// EvaluateAll checks all enabled alert rules and fires webhooks for breached thresholds.
func (s *Service) EvaluateAll(ctx context.Context) (int, error) {
	rules, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return 0, err
	}
	fired := 0
	for _, rule := range rules {
		if rule.LastFiredAt != nil && time.Since(*rule.LastFiredAt) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}
		value, err := s.getCurrentValue(ctx, rule)
		if err != nil {
			s.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("alert_metric_fetch_failed")
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
		if err := s.fireWebhook(ctx, rule.WebhookURL, payload); err != nil {
			s.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("webhook_fire_failed")
			continue
		}
		if err := s.repo.MarkFired(ctx, rule.ID); err != nil {
			s.log.Error().Err(err).Str("rule_id", rule.ID.String()).Msg("mark_fired_failed")
		}
		fired++
	}
	return fired, nil
}

func (s *Service) getCurrentValue(ctx context.Context, rule domain.AlertRule) (float64, error) {
	if s.metrics == nil {
		return 0, nil
	}
	switch rule.Metric {
	case domain.MetricDenyRate:
		return s.metrics.DenyRate(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	case domain.MetricErrorRate:
		return s.metrics.ErrorRate(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	case domain.MetricRateLimitCount:
		return s.metrics.RateLimitedCount(ctx, rule.OrgID, rule.ToolName, rule.WindowSeconds)
	default:
		return 0, nil
	}
}

func (s *Service) fireWebhook(_ context.Context, url string, payload domain.WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
