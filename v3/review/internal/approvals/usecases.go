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

	// Break-glass: verificar que el aprobador no haya decidido antes
	if a.BreakGlass {
		for _, d := range a.Decisions {
			if d.ApproverID == decidedBy {
				return ErrAlreadyDecided
			}
		}

		// Registrar decisión parcial
		a.Decisions = append(a.Decisions, approvaldomain.ApprovalDecision{
			ApproverID: decidedBy, Action: "approve", Note: note, DecidedAt: now,
		})

		approveCount := 0
		for _, d := range a.Decisions {
			if d.Action == "approve" {
				approveCount++
			}
		}

		// ¿Suficientes aprobaciones?
		if approveCount < a.RequiredApprovals {
			// Guardar decisión parcial, no finalizar
			if _, err := u.repo.Update(ctx, a); err != nil {
				return fmt.Errorf("update approval (partial): %w", err)
			}
			u.emitAudit(ctx, a.RequestID, auditdomain.EventApproved, decidedBy,
				fmt.Sprintf("Partial approval (%d/%d): %s", approveCount, a.RequiredApprovals, note),
				map[string]any{"decided_by": decidedBy, "note": note, "approvals": approveCount, "required": a.RequiredApprovals})
			return nil
		}
		// Suficientes — finalizar
	}

	a.Status = approvaldomain.ApprovalStatusApproved
	a.DecidedBy = decidedBy
	a.DecisionNote = note
	a.DecidedAt = &now

	// Si no es break-glass, registrar decisión única
	if !a.BreakGlass {
		a.Decisions = []approvaldomain.ApprovalDecision{
			{ApproverID: decidedBy, Action: "approve", Note: note, DecidedAt: now},
		}
	}

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

	u.emitAudit(ctx, a.RequestID, auditdomain.EventApproved, decidedBy,
		"Approved: "+note, map[string]any{"decided_by": decidedBy, "note": note, "break_glass": a.BreakGlass})

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

	// Break-glass: verificar que no haya decidido antes
	if a.BreakGlass {
		for _, d := range a.Decisions {
			if d.ApproverID == decidedBy {
				return ErrAlreadyDecided
			}
		}
	}

	// Un rechazo siempre finaliza (en break-glass o no)
	a.Decisions = append(a.Decisions, approvaldomain.ApprovalDecision{
		ApproverID: decidedBy, Action: "reject", Note: note, DecidedAt: now,
	})
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

	u.emitAudit(ctx, a.RequestID, auditdomain.EventRejected, decidedBy,
		"Rejected: "+note, map[string]any{"decided_by": decidedBy, "note": note, "break_glass": a.BreakGlass})

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

