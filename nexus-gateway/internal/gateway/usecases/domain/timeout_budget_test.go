package domain

import (
	"testing"
	"time"
)

func TestClampTimeoutMS(t *testing.T) {
	if got := ClampTimeoutMS(0, 10000, 1000, 30000); got != 10000 {
		t.Fatalf("expected 10000 got %d", got)
	}
	if got := ClampTimeoutMS(500, 10000, 1000, 30000); got != 1000 {
		t.Fatalf("expected min clamp 1000 got %d", got)
	}
	if got := ClampTimeoutMS(60000, 10000, 1000, 30000); got != 30000 {
		t.Fatalf("expected max clamp 30000 got %d", got)
	}
}

func TestTimeoutBudgetConsume(t *testing.T) {
	b := NewTimeoutBudget(2000)
	b.Consume("schema", 10*time.Millisecond)
	b.Consume("schema", 20*time.Millisecond)
	if b.StageDurationsMS()["schema"] != 30 {
		t.Fatalf("unexpected stage duration: %+v", b.StageDurationsMS())
	}
}
