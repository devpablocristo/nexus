package alerts

import (
	"testing"

	"github.com/google/uuid"
)

func parseUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", raw, err)
	}
	return id
}

func ptr[T any](value T) *T {
	return &value
}

func strPtr(value string) *string {
	return &value
}
