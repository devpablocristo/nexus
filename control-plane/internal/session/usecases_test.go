package session

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "control-plane/internal/session/usecases/domain"
)

type stubSessionRepo struct {
	sessions map[string]domain.AgentSession
}

func newStubSessionRepo() *stubSessionRepo {
	return &stubSessionRepo{sessions: make(map[string]domain.AgentSession)}
}

func (r *stubSessionRepo) GetOrCreate(_ context.Context, orgID uuid.UUID, sessionID string, actor *string) (domain.AgentSession, error) {
	key := orgID.String() + ":" + sessionID
	if s, ok := r.sessions[key]; ok {
		return s, nil
	}
	s := domain.AgentSession{
		ID:        uuid.New(),
		OrgID:     orgID,
		SessionID: sessionID,
		Actor:     actor,
		CreatedAt: time.Now(),
		LastCallAt: time.Now(),
	}
	r.sessions[key] = s
	return s, nil
}

func (r *stubSessionRepo) IncrementCall(_ context.Context, orgID uuid.UUID, sessionID string, isWrite bool, isDenial bool) error {
	key := orgID.String() + ":" + sessionID
	s := r.sessions[key]
	s.TotalCalls++
	if isWrite {
		s.TotalWrites++
	}
	if isDenial {
		s.TotalDenials++
	}
	s.LastCallAt = time.Now()
	r.sessions[key] = s
	return nil
}

func (r *stubSessionRepo) GetBySessionID(_ context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error) {
	key := orgID.String() + ":" + sessionID
	if s, ok := r.sessions[key]; ok {
		return s, nil
	}
	return domain.AgentSession{}, nil
}

func TestTrackCall(t *testing.T) {
	repo := newStubSessionRepo()
	svc := NewUsecases(repo)

	orgID := uuid.New()
	actor := "bot-1"

	sess, err := svc.TrackCall(context.Background(), orgID, "sess-1", &actor, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %s", sess.SessionID)
	}

	got, _ := repo.GetBySessionID(context.Background(), orgID, "sess-1")
	if got.TotalCalls != 1 {
		t.Errorf("expected 1 call, got %d", got.TotalCalls)
	}
}

func TestTrackCall_Write(t *testing.T) {
	repo := newStubSessionRepo()
	svc := NewUsecases(repo)

	orgID := uuid.New()
	svc.TrackCall(context.Background(), orgID, "sess-2", nil, true, false)

	got, _ := repo.GetBySessionID(context.Background(), orgID, "sess-2")
	if got.TotalWrites != 1 {
		t.Errorf("expected 1 write, got %d", got.TotalWrites)
	}
}

func TestTrackCall_Denial(t *testing.T) {
	repo := newStubSessionRepo()
	svc := NewUsecases(repo)

	orgID := uuid.New()
	svc.TrackCall(context.Background(), orgID, "sess-3", nil, false, true)

	got, _ := repo.GetBySessionID(context.Background(), orgID, "sess-3")
	if got.TotalDenials != 1 {
		t.Errorf("expected 1 denial, got %d", got.TotalDenials)
	}
}

func TestCheckLimits(t *testing.T) {
	svc := NewUsecases(newStubSessionRepo())

	tests := []struct {
		name   string
		sess   domain.AgentSession
		limits domain.SessionLimits
		want   bool
	}{
		{"within limits", domain.AgentSession{TotalCalls: 5, TotalWrites: 2}, domain.SessionLimits{MaxCallsPerSession: 10, MaxWritesPerSession: 5}, true},
		{"calls exceeded", domain.AgentSession{TotalCalls: 10}, domain.SessionLimits{MaxCallsPerSession: 10}, false},
		{"writes exceeded", domain.AgentSession{TotalWrites: 5}, domain.SessionLimits{MaxWritesPerSession: 5}, false},
		{"no limits", domain.AgentSession{TotalCalls: 999}, domain.SessionLimits{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.CheckLimits(tt.sess, tt.limits)
			if got != tt.want {
				t.Errorf("CheckLimits = %v, want %v", got, tt.want)
			}
		})
	}
}
