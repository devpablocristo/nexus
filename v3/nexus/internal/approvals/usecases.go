package approvals

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	approvaldomain "github.com/devpablocristo/nexus/v3/nexus/internal/approvals/usecases/domain"
	auditdomain "github.com/devpablocristo/nexus/v3/nexus/internal/audit/usecases/domain"
	"github.com/devpablocristo/nexus/v3/nexus/internal/callbacks"
	requestdomain "github.com/devpablocristo/nexus/v3/nexus/internal/requests/usecases/domain"
	"github.com/google/uuid"
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
	callbacks   callbacks.ApprovalPublisher
}

func NewUsecases(repo Repository, requestRepo RequestUpdater) *Usecases {
	return &Usecases{repo: repo, requestRepo: requestRepo}
}

// WithAuditSink inyecta el audit sink (patrón builder de v2).
func (u *Usecases) WithAuditSink(sink AuditSink) *Usecases {
	u.audit = sink
	return u
}

func (u *Usecases) WithApprovalCallbacks(publisher callbacks.ApprovalPublisher) *Usecases {
	u.callbacks = publisher
	return u
}

func (u *Usecases) ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error) {
	return u.repo.ListPending(ctx, limit)
}

func (u *Usecases) GetByID(ctx context.Context, approvalID uuid.UUID) (approvaldomain.Approval, error) {
	return u.repo.GetByID(ctx, approvalID)
}

func (u *Usecases) Approve(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	decidedBy = strings.TrimSpace(decidedBy)
	if decidedBy == "" {
		return ErrActorRequired
	}
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("get approval: %w", err)
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}

	now := time.Now().UTC()
	if !a.ExpiresAt.IsZero() && !now.Before(a.ExpiresAt) {
		return ErrExpired
	}

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
	u.emitApprovalResolved(ctx, req, a)

	return nil
}

func (u *Usecases) Reject(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	decidedBy = strings.TrimSpace(decidedBy)
	if decidedBy == "" {
		return ErrActorRequired
	}
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("get approval: %w", err)
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}

	now := time.Now().UTC()
	if !a.ExpiresAt.IsZero() && !now.Before(a.ExpiresAt) {
		return ErrExpired
	}

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
	u.emitApprovalResolved(ctx, req, a)

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

func (u *Usecases) emitApprovalResolved(ctx context.Context, req requestdomain.Request, approval approvaldomain.Approval) {
	if u.callbacks == nil {
		return
	}
	if err := u.callbacks.Publish(ctx, callbacks.ApprovalEvent{
		Event:        callbacks.EventApprovalResolved,
		ApprovalID:   approval.ID.String(),
		OrgID:        stringOrEmpty(req.OrgID, approval.OrgID),
		RequestID:    approval.RequestID.String(),
		Decision:     string(approval.Status),
		DecidedBy:    strings.TrimSpace(approval.DecidedBy),
		DecisionNote: strings.TrimSpace(approval.DecisionNote),
		DecidedAt:    timePtrRFC3339(approval.DecidedAt),
	}); err != nil {
		slog.Error("approval callback publish failed", "event", callbacks.EventApprovalResolved, "request_id", approval.RequestID, "error", err)
	}
}

func stringOrEmpty(values ...*string) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func timePtrRFC3339(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}
