package sentry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type InMemoryState struct {
	mu           sync.Mutex
	baselines    map[string]Baseline
	fingerprints map[string]FingerprintState
	dataDir      string
}

func NewInMemoryState() *InMemoryState {
	return &InMemoryState{
		baselines:    map[string]Baseline{},
		fingerprints: map[string]FingerprintState{},
	}
}

func NewFileBackedState(dataDir string) *InMemoryState {
	s := &InMemoryState{
		baselines:    map[string]Baseline{},
		fingerprints: map[string]FingerprintState{},
		dataDir:      dataDir,
	}
	s.load()
	return s
}

func (s *InMemoryState) GetBaseline(_ context.Context, orgID uuid.UUID, toolName, metric string) (Baseline, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := orgID.String() + "|" + toolName + "|" + metric
	if b, ok := s.baselines[key]; ok {
		return b, nil
	}
	return Baseline{OrgID: orgID, ToolName: toolName, Metric: metric}, nil
}

func (s *InMemoryState) UpsertBaseline(_ context.Context, b Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := b.OrgID.String() + "|" + b.ToolName + "|" + b.Metric
	s.baselines[key] = b
	s.persist()
	return nil
}

func (s *InMemoryState) GetFingerprint(_ context.Context, orgID uuid.UUID, fingerprint string) (FingerprintState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := orgID.String() + "|" + fingerprint
	if f, ok := s.fingerprints[key]; ok {
		return f, nil
	}
	return FingerprintState{OrgID: orgID, Fingerprint: fingerprint}, nil
}

func (s *InMemoryState) UpsertFingerprint(_ context.Context, f FingerprintState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := f.OrgID.String() + "|" + f.Fingerprint
	s.fingerprints[key] = f
	s.persist()
	return nil
}

type stateSnapshot struct {
	Baselines    map[string]Baseline          `json:"baselines"`
	Fingerprints map[string]FingerprintState  `json:"fingerprints"`
}

func (s *InMemoryState) load() {
	if s.dataDir == "" {
		return
	}
	path := filepath.Join(s.dataDir, "sentry_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var snap stateSnapshot
	if json.Unmarshal(data, &snap) != nil {
		return
	}
	if snap.Baselines != nil {
		s.baselines = snap.Baselines
	}
	if snap.Fingerprints != nil {
		s.fingerprints = snap.Fingerprints
	}
}

func (s *InMemoryState) persist() {
	if s.dataDir == "" {
		return
	}
	_ = os.MkdirAll(s.dataDir, 0o755)
	snap := stateSnapshot{
		Baselines:    s.baselines,
		Fingerprints: s.fingerprints,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	tmp := filepath.Join(s.dataDir, "sentry_state.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(s.dataDir, "sentry_state.json"))
}
