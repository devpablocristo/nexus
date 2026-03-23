package tasks

import (
	"fmt"
	"strings"
	"sync"

	"github.com/devpablocristo/core/backend/go/fsm"

	"github.com/devpablocristo/core/governance/go/reviewclient"
	domain "github.com/devpablocristo/nexus/v3/companion/internal/tasks/usecases/domain"
)

// Eventos de transición de tarea (valores opacos para la FSM).
const (
	evInvestigate           = "investigate"
	evReviewPendingApproval = "review_pending_approval"
	evReviewResolvedAllow   = "review_resolved_allow"
	evReviewResolvedDeny    = "review_resolved_deny"
)

var companionTaskMachine = sync.OnceValue(buildCompanionTaskFSM)

func buildCompanionTaskFSM() *fsm.Machine[string, string] {
	return fsm.New([]fsm.Rule[string, string]{
		{From: domain.TaskStatusNew, Event: evInvestigate, To: domain.TaskStatusInvestigating},
		{From: domain.TaskStatusInvestigating, Event: evInvestigate, To: domain.TaskStatusInvestigating},

		{From: domain.TaskStatusNew, Event: evReviewPendingApproval, To: domain.TaskStatusWaitingForApproval},
		{From: domain.TaskStatusInvestigating, Event: evReviewPendingApproval, To: domain.TaskStatusWaitingForApproval},

		{From: domain.TaskStatusNew, Event: evReviewResolvedAllow, To: domain.TaskStatusDone},
		{From: domain.TaskStatusInvestigating, Event: evReviewResolvedAllow, To: domain.TaskStatusDone},
		{From: domain.TaskStatusWaitingForApproval, Event: evReviewResolvedAllow, To: domain.TaskStatusDone},

		{From: domain.TaskStatusNew, Event: evReviewResolvedDeny, To: domain.TaskStatusFailed},
		{From: domain.TaskStatusInvestigating, Event: evReviewResolvedDeny, To: domain.TaskStatusFailed},
		{From: domain.TaskStatusWaitingForApproval, Event: evReviewResolvedDeny, To: domain.TaskStatusFailed},
	})
}

func eventFromSubmitResponse(sub reviewclient.SubmitResponse) (string, error) {
	s := strings.ToLower(strings.TrimSpace(sub.Status))
	switch s {
	case "allowed", "approved", "executed":
		return evReviewResolvedAllow, nil
	case "denied", "rejected":
		return evReviewResolvedDeny, nil
	case "pending_approval":
		return evReviewPendingApproval, nil
	default:
		return "", fmt.Errorf("unexpected review status after submit: %q", sub.Status)
	}
}

// eventFromReviewRequestStatus mapea estado HTTP de Review a evento FSM; apply=false = sin cambio.
func eventFromReviewRequestStatus(status string) (event string, apply bool) {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "pending_approval", "pending", "evaluated":
		return "", false
	case "allowed", "approved", "executed":
		return evReviewResolvedAllow, true
	case "denied", "rejected", "expired", "failed", "cancelled":
		return evReviewResolvedDeny, true
	default:
		return "", false
	}
}
