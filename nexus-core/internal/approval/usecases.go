package approval

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	domain "nexus-core/internal/approval/usecases/domain"
	auditdomain "nexus-core/internal/audit/usecases/domain"
	"nexus/pkg/types"
)

type RepoPort interface {
	Create(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error)
	ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error)
	ListByIntent(ctx context.Context, orgID, intentID uuid.UUID) ([]domain.PendingApproval, error)
	Decide(ctx context.Context, orgID, id uuid.UUID, status domain.Status, decidedBy string) error
	RejectPendingByIntent(ctx context.Context, orgID, intentID uuid.UUID, decidedBy string) error
	ExpireOld(ctx context.Context) (int64, error)
}

type Usecases struct {
	repo    RepoPort
	intents IntentStatusPort
	audit   AuditPort
}

type IntentStatusPort interface {
	MarkIntentApproved(ctx context.Context, orgID, intentID uuid.UUID) error
	MarkIntentRejected(ctx context.Context, orgID, intentID uuid.UUID) error
}

type AuditPort interface {
	Create(ctx context.Context, ev auditdomain.AuditEvent) error
}

func NewUsecases(repo RepoPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) WithIntentPort(port IntentStatusPort) *Usecases {
	u.intents = port
	return u
}

func (u *Usecases) WithAuditPort(port AuditPort) *Usecases {
	u.audit = port
	return u
}

func (u *Usecases) RequestApproval(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 3600
	}
	if req.ApprovalMode == "" {
		req.ApprovalMode = domain.ApprovalModeStandard
	}
	if req.ApprovalStep <= 0 {
		req.ApprovalStep = 1
	}
	if req.ApprovalStepsTotal <= 0 {
		req.ApprovalStepsTotal = 1
	}
	item, err := u.repo.Create(ctx, req)
	if err != nil {
		return domain.PendingApproval{}, err
	}
	u.auditApprovalEvent(ctx, item, auditdomain.DecisionDeny, auditdomain.StatusBlocked, "approval_requested", nil, map[string]any{
		"approval_mode":        string(item.ApprovalMode),
		"approval_step":        item.ApprovalStep,
		"approval_steps_total": item.ApprovalStepsTotal,
		"approval_group_id":    uuidToString(item.ApprovalGroupID),
		"intent_id":            uuidToString(item.IntentID),
	})
	return item, nil
}

func (u *Usecases) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error) {
	return u.repo.ListPending(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	return u.repo.GetByID(ctx, orgID, id)
}

func (u *Usecases) Approve(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if item.ApprovalMode == domain.ApprovalModeBreakGlass && item.IntentID != nil {
		related, err := u.repo.ListByIntent(ctx, orgID, *item.IntentID)
		if err != nil {
			return err
		}
		for _, approval := range related {
			if approval.ID != item.ID && approval.Status == domain.StatusApproved && approval.DecidedBy != nil && *approval.DecidedBy == decidedBy {
				return types.NewHTTPError(http.StatusConflict, types.ErrCodeApprovalRequired, "break-glass requires two distinct approvers")
			}
		}
	}
	if err := u.repo.Decide(ctx, orgID, id, domain.StatusApproved, decidedBy); err != nil {
		return err
	}
	if item.IntentID != nil && u.intents != nil {
		if item.ApprovalMode == domain.ApprovalModeBreakGlass {
			related, err := u.repo.ListByIntent(ctx, orgID, *item.IntentID)
			if err != nil {
				return err
			}
			approvedCount := 1
			for _, approval := range related {
				if approval.ID != item.ID && approval.Status == domain.StatusApproved {
					approvedCount++
				}
			}
			u.auditApprovalEvent(ctx, item, auditdomain.DecisionAllow, auditdomain.StatusSuccess, "break_glass_stage_approved", strPtr(decidedBy), map[string]any{
				"approved_count":       approvedCount,
				"approval_steps_total": item.ApprovalStepsTotal,
				"intent_id":            item.IntentID.String(),
			})
			if approvedCount < item.ApprovalStepsTotal {
				return nil
			}
			u.auditApprovalEvent(ctx, item, auditdomain.DecisionAllow, auditdomain.StatusSuccess, "break_glass_granted", strPtr(decidedBy), map[string]any{
				"approved_count":       approvedCount,
				"approval_steps_total": item.ApprovalStepsTotal,
				"intent_id":            item.IntentID.String(),
			})
			return u.intents.MarkIntentApproved(ctx, orgID, *item.IntentID)
		}
		u.auditApprovalEvent(ctx, item, auditdomain.DecisionAllow, auditdomain.StatusSuccess, "approval_approved", strPtr(decidedBy), nil)
		return u.intents.MarkIntentApproved(ctx, orgID, *item.IntentID)
	}
	u.auditApprovalEvent(ctx, item, auditdomain.DecisionAllow, auditdomain.StatusSuccess, "approval_approved", strPtr(decidedBy), nil)
	return nil
}

func (u *Usecases) Reject(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if item.IntentID != nil && u.intents != nil {
		if item.ApprovalMode == domain.ApprovalModeBreakGlass {
			if err := u.repo.RejectPendingByIntent(ctx, orgID, *item.IntentID, decidedBy); err != nil {
				return err
			}
			u.auditApprovalEvent(ctx, item, auditdomain.DecisionDeny, auditdomain.StatusBlocked, "break_glass_rejected", strPtr(decidedBy), map[string]any{
				"intent_id": item.IntentID.String(),
			})
			return u.intents.MarkIntentRejected(ctx, orgID, *item.IntentID)
		}
		if err := u.repo.Decide(ctx, orgID, id, domain.StatusRejected, decidedBy); err != nil {
			return err
		}
		u.auditApprovalEvent(ctx, item, auditdomain.DecisionDeny, auditdomain.StatusBlocked, "approval_rejected", strPtr(decidedBy), nil)
		return u.intents.MarkIntentRejected(ctx, orgID, *item.IntentID)
	}
	if err := u.repo.Decide(ctx, orgID, id, domain.StatusRejected, decidedBy); err != nil {
		return err
	}
	u.auditApprovalEvent(ctx, item, auditdomain.DecisionDeny, auditdomain.StatusBlocked, "approval_rejected", strPtr(decidedBy), nil)
	return nil
}

func (u *Usecases) ExpireOld(ctx context.Context) (int64, error) {
	return u.repo.ExpireOld(ctx)
}

func (u *Usecases) auditApprovalEvent(ctx context.Context, item domain.PendingApproval, decision auditdomain.Decision, status auditdomain.Status, reason string, decidedBy *string, output map[string]any) {
	if u.audit == nil {
		return
	}
	if output == nil {
		output = map[string]any{}
	}
	if decidedBy != nil && *decidedBy != "" {
		output["decided_by"] = *decidedBy
	}
	rc := reason
	_ = u.audit.Create(ctx, auditdomain.AuditEvent{
		OrgID:           item.OrgID,
		ToolID:          item.ToolID,
		ToolName:        item.ToolName,
		RequestID:       item.RequestID,
		Actor:           item.Actor,
		ActorRole:       item.Role,
		InputRedacted:   item.InputRedacted,
		ContextRedacted: item.ContextRedacted,
		Decision:        decision,
		PolicyID:        item.PolicyID,
		Reason:          &rc,
		Status:          status,
		OutputRedacted:  output,
	})
}

func uuidToString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func strPtr(v string) *string {
	return &v
}
