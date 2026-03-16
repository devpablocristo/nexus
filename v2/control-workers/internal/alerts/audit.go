package alerts

import (
	"context"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	sharedobservability "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/observability"
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
			"control-workers alert audit failed",
			"event_type", req.EventType,
			"action_id", req.ActionID,
			"error", err,
		)
	}
}
