// Package sentry implements anomaly detection worker and state handling.
package sentry

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type InMemoryState struct {
	mu           sync.Mutex
	baselines    map[string]Baseline
	fingerprints map[string]FingerprintState
}

func NewInMemoryState() *InMemoryState {
	return &InMemoryState{
		baselines:    map[string]Baseline{},
		fingerprints: map[string]FingerprintState{},
	}
}

func (s *InMemoryState) GetBaseline(ctx context.Context, orgID uuid.UUID, toolName, metric string) (Baseline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := orgID.String() + "|" + toolName + "|" + metric
	if b, ok := s.baselines[key]; ok {
		return b, nil
	}
	return Baseline{OrgID: orgID, ToolName: toolName, Metric: metric}, nil
}

func (s *InMemoryState) UpsertBaseline(ctx context.Context, b Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := b.OrgID.String() + "|" + b.ToolName + "|" + b.Metric
	s.baselines[key] = b
	return nil
}

func (s *InMemoryState) GetFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (FingerprintState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := orgID.String() + "|" + fingerprint
	if f, ok := s.fingerprints[key]; ok {
		return f, nil
	}
	return FingerprintState{OrgID: orgID, Fingerprint: fingerprint}, nil
}

func (s *InMemoryState) UpsertFingerprint(ctx context.Context, f FingerprintState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := f.OrgID.String() + "|" + f.Fingerprint
	s.fingerprints[key] = f
	return nil
}
