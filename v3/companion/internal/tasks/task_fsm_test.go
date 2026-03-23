package tasks

import (
	"testing"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

func TestEventFromSubmitResponse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status string
		want   string
	}{
		{"allowed", evReviewResolvedAllow},
		{"ALLOWED", evReviewResolvedAllow},
		{"denied", evReviewResolvedDeny},
		{"pending_approval", evReviewPendingApproval},
	}
	for _, tc := range cases {
		ev, err := eventFromSubmitResponse(reviewclient.SubmitResponse{Status: tc.status})
		if err != nil || ev != tc.want {
			t.Fatalf("status %q: got %q %v want %q", tc.status, ev, err, tc.want)
		}
	}
	_, err := eventFromSubmitResponse(reviewclient.SubmitResponse{Status: "weird"})
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestEventFromReviewRequestStatus(t *testing.T) {
	t.Parallel()
	ev, ok := eventFromReviewRequestStatus("pending_approval")
	if ok || ev != "" {
		t.Fatalf("pending: got %q %v", ev, ok)
	}
	ev, ok = eventFromReviewRequestStatus("approved")
	if !ok || ev != evReviewResolvedAllow {
		t.Fatalf("approved: got %q %v", ev, ok)
	}
	ev, ok = eventFromReviewRequestStatus("rejected")
	if !ok || ev != evReviewResolvedDeny {
		t.Fatalf("rejected: got %q %v", ev, ok)
	}
}

func TestCompanionTaskFSM_investigateAndReview(t *testing.T) {
	t.Parallel()
	m := companionTaskMachine()
	to, err := m.Transition(domain.TaskStatusNew, evInvestigate)
	if err != nil || to != domain.TaskStatusInvestigating {
		t.Fatalf("investigate: %q %v", to, err)
	}
	to, err = m.Transition(domain.TaskStatusInvestigating, evInvestigate)
	if err != nil || to != domain.TaskStatusInvestigating {
		t.Fatalf("investigate idempotent: %q %v", to, err)
	}
	to, err = m.Transition(domain.TaskStatusInvestigating, evReviewPendingApproval)
	if err != nil || to != domain.TaskStatusWaitingForApproval {
		t.Fatalf("pending: %q %v", to, err)
	}
	to, err = m.Transition(domain.TaskStatusInvestigating, evReviewResolvedAllow)
	if err != nil || to != domain.TaskStatusDone {
		t.Fatalf("allow from investigating: %q %v", to, err)
	}
}
