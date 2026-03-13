package circuitbreaker

import (
	"testing"
	"time"
)

func TestClosedToOpen(t *testing.T) {
	b := New(Config{FailureThreshold: 3, HalfOpenMax: 1, ResetTimeout: 50 * time.Millisecond})

	if !b.Allow() {
		t.Fatal("closed breaker should allow")
	}

	b.RecordFailure()
	b.RecordFailure()
	if b.State() != StateClosed {
		t.Fatal("should still be closed after 2 failures")
	}

	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatal("should be open after 3 failures")
	}

	if b.Allow() {
		t.Fatal("open breaker should reject")
	}
}

func TestOpenToHalfOpenToClose(t *testing.T) {
	b := New(Config{FailureThreshold: 2, HalfOpenMax: 1, ResetTimeout: 30 * time.Millisecond})

	b.RecordFailure()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatal("expected open")
	}

	time.Sleep(40 * time.Millisecond)

	if !b.Allow() {
		t.Fatal("should allow after reset timeout (half_open)")
	}
	if b.State() != StateHalfOpen {
		t.Fatal("expected half_open")
	}

	b.RecordSuccess()
	if b.State() != StateClosed {
		t.Fatal("should close after half_open success")
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	b := New(Config{FailureThreshold: 1, HalfOpenMax: 2, ResetTimeout: 20 * time.Millisecond})

	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatal("expected open")
	}

	time.Sleep(25 * time.Millisecond)
	b.Allow()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatal("half_open failure should reopen")
	}
}

func TestSuccessResetsClosed(t *testing.T) {
	b := New(Config{FailureThreshold: 3, HalfOpenMax: 1, ResetTimeout: time.Second})

	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess()

	b.RecordFailure()
	if b.State() != StateClosed {
		t.Fatal("success should have reset failure count")
	}
}
