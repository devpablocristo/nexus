package action

import (
	"context"
	"log"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"

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
		log.Printf(
			"action audit sink failed: event_type=%s action_id=%s resource_id=%s err=%v payload=%+v",
			req.EventType,
			req.ActionID,
			req.ResourceID,
			err,
			req,
		)
	}
}

func actionAuditActor(actor actiondomain.ActorRef) *sharedaudit.Actor {
	return &sharedaudit.Actor{Type: string(actor.Type), ID: actor.ID}
}
