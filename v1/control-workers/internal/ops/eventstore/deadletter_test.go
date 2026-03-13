package eventstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeadLetterLog_Append(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dead_letters.jsonl")
	log := NewDeadLetterLog(path)

	err := log.Append(DeadLetterEntry{
		EventID:  "evt-1",
		Payload:  map[string]any{"foo": "bar"},
		Error:    "boom",
		Attempts: 3,
		FailedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dlq file: %v", err)
	}
	if got := strings.TrimSpace(string(raw)); got == "" {
		t.Fatalf("expected non-empty jsonl line")
	}
}
