package gateway

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

func TestInMemoryLeaseRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryLeaseRepository()
	intentID := uuid.New()

	lease, err := repo.Create(context.Background(), gwdomain.ExecutionLease{
		IntentID:  intentID,
		ToolName:  "echo",
		RiskClass: gwdomain.RiskClassMutateProd,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if lease.ID == uuid.Nil {
		t.Fatal("expected lease id")
	}
	if lease.Status != gwdomain.ExecutionLeaseStatusActive {
		t.Fatalf("unexpected initial lease status: %s", lease.Status)
	}

	got, err := repo.GetByID(context.Background(), lease.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if got.IntentID != intentID {
		t.Fatalf("unexpected lease from GetByID: %#v", got)
	}

	if _, err := repo.Consume(context.Background(), lease.ID, intentID); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	used, err := repo.GetByID(context.Background(), lease.ID)
	if err != nil {
		t.Fatalf("GetByID after Consume returned error: %v", err)
	}
	if used.Status != gwdomain.ExecutionLeaseStatusUsed || used.UsedAt == nil {
		t.Fatalf("unexpected used lease state: %#v", used)
	}

	expiring, err := repo.Create(context.Background(), gwdomain.ExecutionLease{
		IntentID:  intentID,
		ToolName:  "echo",
		RiskClass: gwdomain.RiskClassMutateProd,
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("Create expiring lease returned error: %v", err)
	}
	if _, err := repo.Consume(context.Background(), expiring.ID, intentID); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("Consume expired lease error = %v, want %v", err, ErrLeaseExpired)
	}
	expired, err := repo.GetByID(context.Background(), expiring.ID)
	if err != nil {
		t.Fatalf("GetByID after expired Consume returned error: %v", err)
	}
	if expired.Status != gwdomain.ExecutionLeaseStatusExpired {
		t.Fatalf("unexpected expired lease state: %#v", expired)
	}
}

func TestInMemoryLeaseRepositoryConsumeIsAtomic(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryLeaseRepository()
	intentID := uuid.New()

	lease, err := repo.Create(context.Background(), gwdomain.ExecutionLease{
		IntentID:  intentID,
		ToolName:  "echo",
		RiskClass: gwdomain.RiskClassMutateProd,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	errs := make(chan error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Consume(context.Background(), lease.ID, intentID)
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	var successCount int
	var inactiveCount int
	for err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrLeaseNotActive):
			inactiveCount++
		default:
			t.Fatalf("unexpected consume error: %v", err)
		}
	}

	if successCount != 1 || inactiveCount != 1 {
		t.Fatalf("unexpected consume outcomes: success=%d inactive=%d", successCount, inactiveCount)
	}
}
