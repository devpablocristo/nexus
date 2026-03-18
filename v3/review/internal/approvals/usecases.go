package approvals

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	approvaldomain "github.com/devpablocristo/nexus/v3/review/internal/approvals/usecases/domain"
	auditdomain "github.com/devpablocristo/nexus/v3/review/internal/audit/usecases/domain"
	requestdomain "github.com/devpablocristo/nexus/v3/review/internal/requests/usecases/domain"
)

type RequestUpdater interface {
	GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error)
	Update(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
}

// AuditSink emite eventos al audit trail (best-effort).
type AuditSink interface {
	AppendEvent(ctx context.Context, requestID uuid.UUID, eventType, actorType, actorID, summary string, data map[string]any) error
}

type Usecases struct {
	repo        Repository
	requestRepo RequestUpdater
	audit       AuditSink
}

func NewUsecases(repo Repository, requestRepo RequestUpdater) *Usecases {
	return &Usecases{repo: repo, requestRepo: requestRepo}
}

// WithAuditSink inyecta el audit sink (patrón builder de v2).
func (u *Usecases) WithAuditSink(sink AuditSink) *Usecases {
	u.audit = sink
	return u
}

func (u *Usecases) ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error) {
	return u.repo.ListPending(ctx, limit)
}

func (u *Usecases) Approve(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("get approval: %w", err)
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}
	now := time.Now().UTC()
	a.Status = approvaldomain.ApprovalStatusApproved
	a.DecidedBy = decidedBy
	a.DecisionNote = note
	a.DecidedAt = &now
	if _, err := u.repo.Update(ctx, a); err != nil {
		return fmt.Errorf("update approval: %w", err)
	}
	req, err := u.requestRepo.GetByID(ctx, a.RequestID)
	if err != nil {
		return fmt.Errorf("get request for approval: %w", err)
	}
	req.Status = requestdomain.StatusApproved
	req.DecidedAt = &now
	req.UpdatedAt = now
	if _, err := u.requestRepo.Update(ctx, req); err != nil {
		return fmt.Errorf("update request status: %w", err)
	}

	// Audit: best-effort (patrón v2)
	u.emitAudit(ctx, a.RequestID, auditdomain.EventApproved, decidedBy,
		"Approved: "+note, map[string]any{"decided_by": decidedBy, "note": note})

	return nil
}

func (u *Usecases) Reject(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("get approval: %w", err)
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}
	now := time.Now().UTC()
	a.Status = approvaldomain.ApprovalStatusRejected
	a.DecidedBy = decidedBy
	a.DecisionNote = note
	a.DecidedAt = &now
	if _, err := u.repo.Update(ctx, a); err != nil {
		return fmt.Errorf("update approval: %w", err)
	}
	req, err := u.requestRepo.GetByID(ctx, a.RequestID)
	if err != nil {
		return fmt.Errorf("get request for rejection: %w", err)
	}
	req.Status = requestdomain.StatusRejected
	req.DecidedAt = &now
	req.UpdatedAt = now
	if _, err := u.requestRepo.Update(ctx, req); err != nil {
		return fmt.Errorf("update request status: %w", err)
	}

	// Audit: best-effort
	u.emitAudit(ctx, a.RequestID, auditdomain.EventRejected, decidedBy,
		"Rejected: "+note, map[string]any{"decided_by": decidedBy, "note": note})

	return nil
}

// emitAudit emite un evento al audit trail. Best-effort: loguea error, nunca falla la operación.
func (u *Usecases) emitAudit(ctx context.Context, requestID uuid.UUID, eventType, actorID, summary string, data map[string]any) {
	if u.audit == nil {
		return
	}
	if err := u.audit.AppendEvent(ctx, requestID, eventType, "human", actorID, summary, data); err != nil {
		slog.Error("audit event failed", "error", err, "request_id", requestID, "event", eventType)
	}
}

