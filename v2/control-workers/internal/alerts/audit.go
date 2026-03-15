package alerts

import (
	"context"
	"log"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
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
			"control-workers alert audit failed: event_type=%s source_id=%s err=%v payload=%+v",
			req.EventType,
			req.ActionID,
			err,
			req,
		)
	}
}
