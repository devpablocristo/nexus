package audit

import (
	"testing"

	"github.com/google/uuid"

	auditdomain "data-plane/internal/audit/usecases/domain"
)

func TestComputeEventHash_ChangesWithPrev(t *testing.T) {
	ev := auditdomain.AuditEvent{
		OrgID:     uuid.New(),
		ToolID:    uuid.New(),
		ToolName:  "echo",
		RequestID: "r1",
		Decision:  auditdomain.DecisionAllow,
		Status:    auditdomain.StatusSuccess,
		LatencyMS: 12,
	}

	h1, err := computeEventHash(nil, ev, []byte(`{}`), []byte(`{}`), []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}
	prev := h1
	h2, err := computeEventHash(&prev, ev, []byte(`{}`), []byte(`{}`), []byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected different hashes when prev changes")
	}
}
