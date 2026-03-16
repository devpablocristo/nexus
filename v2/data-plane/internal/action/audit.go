package action

import (
	"context"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type AuditSink interface {
	Create(ctx context.Context, req sharedaudit.WriteRequest) error
}

func (u *Usecases) WithAuditSink(sink AuditSink) *Usecases {
	u.audit = sink
	return u
}

func (u *Usecases) emitAudit(ctx context.Context, req sharedaudit.WriteRequest) {
	if u.audit == nil {
		return
	}
	if err := u.audit.Create(ctx, req); err != nil {
		sharedobservability.LoggerFromContext(ctx).Error(
			"action audit sink failed",
			"event_type", req.EventType,
			"action_id", req.ActionID,
			"resource_id", req.ResourceID,
			"error", err,
		)
	}
}

func actionAuditActor(actor actiondomain.ActorRef) *sharedaudit.Actor {
	return &sharedaudit.Actor{Type: string(actor.Type), ID: actor.ID}
}
