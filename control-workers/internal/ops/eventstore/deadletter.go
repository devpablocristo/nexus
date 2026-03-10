package eventstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeadLetterEntry represents a permanently failed event persisted for replay/forensics.
type DeadLetterEntry struct {
	EventID  string    `json:"event_id"`
	Payload  any       `json:"payload"`
	Error    string    `json:"error"`
	Attempts int       `json:"attempts"`
	FailedAt time.Time `json:"failed_at"`
}

// DeadLetterLog stores failed events as JSONL.
type DeadLetterLog struct {
	mu   sync.Mutex
	path string
}

// NewDeadLetterLog creates a file-backed dead-letter logger.
func NewDeadLetterLog(path string) *DeadLetterLog {
	return &DeadLetterLog{path: path}
}

// Append writes one failed event line in JSONL format.
func (d *DeadLetterLog) Append(entry DeadLetterEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	_, err = f.Write(line)
	return err
}
