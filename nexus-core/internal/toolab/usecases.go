package toolab

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	domain "nexus-core/internal/toolab/usecases/domain"
)

// Service defines the business operations for the toolab adapter.
type Service interface {
	Manifest() (appName, appVersion, adapterVersion string, capabilities []string)
	Fingerprint(ctx context.Context) (string, error)
	Snapshot(ctx context.Context, label string) (*domain.SnapshotMeta, error)
	Restore(ctx context.Context, snapshotID string) (*domain.SnapshotMeta, error)
	Reset(ctx context.Context) (string, error)
	Metrics(ctx context.Context) ([]domain.MetricItem, error)
}

// Config holds adapter configuration provided at startup.
type Config struct {
	AppVersion string
}

type service struct {
	repo RepositoryPort
	cfg  Config

	mu        sync.RWMutex
	snapshots map[string]domain.SnapshotMeta
}

// NewService creates the toolab adapter service.
func NewService(repo RepositoryPort, cfg Config) Service {
	return &service{
		repo:      repo,
		cfg:       cfg,
		snapshots: make(map[string]domain.SnapshotMeta),
	}
}

func (s *service) Manifest() (string, string, string, []string) {
	return "nexus", s.cfg.AppVersion, "1", []string{
		"state.fingerprint",
		"state.snapshot",
		"state.restore",
		"state.reset",
		"metrics",
	}
}

func (s *service) Fingerprint(ctx context.Context) (string, error) {
	return s.repo.Fingerprint(ctx)
}

func (s *service) Snapshot(ctx context.Context, label string) (*domain.SnapshotMeta, error) {
	fp, err := s.repo.Fingerprint(ctx)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("snap_%s", time.Now().UTC().Format("20060102_150405"))
	if err := s.repo.CreateSavepoint(ctx, id); err != nil {
		return nil, err
	}

	meta := domain.SnapshotMeta{
		ID:          id,
		Fingerprint: fp,
		Label:       label,
		CreatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.snapshots[id] = meta
	s.mu.Unlock()

	return &meta, nil
}

func (s *service) Restore(ctx context.Context, snapshotID string) (*domain.SnapshotMeta, error) {
	s.mu.RLock()
	meta, ok := s.snapshots[snapshotID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	if err := s.repo.RollbackToSavepoint(ctx, snapshotID); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *service) Reset(ctx context.Context) (string, error) {
	if err := s.repo.TruncateAll(ctx); err != nil {
		return "", err
	}
	fp, _ := s.repo.Fingerprint(ctx)
	return fp, nil
}

func (s *service) Metrics(_ context.Context) ([]domain.MetricItem, error) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return nil, err
	}

	var items []domain.MetricItem
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			labels := make(map[string]string, len(m.GetLabel()))
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}

			item := domain.MetricItem{
				Name:   mf.GetName(),
				Type:   metricTypeName(mf.GetType()),
				Labels: labels,
			}

			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				item.Value = m.GetCounter().GetValue()
			case dto.MetricType_GAUGE:
				item.Value = m.GetGauge().GetValue()
			case dto.MetricType_HISTOGRAM:
				h := m.GetHistogram()
				buckets := make([]map[string]any, 0, len(h.GetBucket()))
				for _, b := range h.GetBucket() {
					buckets = append(buckets, map[string]any{
						"upper_bound":      b.GetUpperBound(),
						"cumulative_count": b.GetCumulativeCount(),
					})
				}
				item.Value = map[string]any{
					"count":   h.GetSampleCount(),
					"sum":     h.GetSampleSum(),
					"buckets": buckets,
				}
			case dto.MetricType_SUMMARY:
				sm := m.GetSummary()
				quantiles := make([]map[string]any, 0, len(sm.GetQuantile()))
				for _, q := range sm.GetQuantile() {
					quantiles = append(quantiles, map[string]any{
						"quantile": q.GetQuantile(),
						"value":    q.GetValue(),
					})
				}
				item.Value = map[string]any{
					"count":     sm.GetSampleCount(),
					"sum":       sm.GetSampleSum(),
					"quantiles": quantiles,
				}
			default:
				item.Value = m.GetUntyped().GetValue()
			}

			items = append(items, item)
		}
	}
	return items, nil
}

func metricTypeName(t dto.MetricType) string {
	switch t {
	case dto.MetricType_COUNTER:
		return "counter"
	case dto.MetricType_GAUGE:
		return "gauge"
	case dto.MetricType_HISTOGRAM:
		return "histogram"
	case dto.MetricType_SUMMARY:
		return "summary"
	default:
		return "untyped"
	}
}
