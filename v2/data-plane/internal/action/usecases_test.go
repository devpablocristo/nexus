package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sharedaudit "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/audit"
	"github.com/google/uuid"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type stubIncidentSink struct {
	items []IncidentRequest
	err   error
}

func (s *stubIncidentSink) Create(_ context.Context, req IncidentRequest) error {
	s.items = append(s.items, req)
	return s.err
}

type stubAuditSink struct {
	items []sharedaudit.WriteRequest
	err   error
}

func (s *stubAuditSink) Create(_ context.Context, req sharedaudit.WriteRequest) error {
	s.items = append(s.items, req)
	return s.err
}

type stubExecutor struct {
	result map[string]any
	err    error
}

func (s stubExecutor) Execute(_ context.Context, _ actiondomain.Action, _ actiondomain.ActorRef) (map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	return cloneMap(s.result), nil
}

type stubResourceResolver struct {
	resource actiondomain.ProtectedResource
	err      error
}

func (s stubResourceResolver) GetByID(_ context.Context, _ string) (actiondomain.ProtectedResource, error) {
	if s.err != nil {
		return actiondomain.ProtectedResource{}, s.err
	}
	return s.resource, nil
}

type stubPolicySource struct {
	items         []ActionPolicy
	err           error
	gotActionType string
	gotResource   string
}

func (s *stubPolicySource) List(_ context.Context, actionType, resourceType string) ([]ActionPolicy, error) {
	s.gotActionType = actionType
	s.gotResource = resourceType
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func validCreateRequest() CreateRequest {
	return CreateRequest{
		ActionType:    actiondomain.ActionTypeWithdrawal,
		ResourceID:    "wallet_hot_usdc_1",
		ResourceType:  actiondomain.ResourceTypeWallet,
		SourceSystem:  "treasury-orchestrator",
		Justification: "Daily settlement withdrawal",
		RequestedBy:   actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "treasury-bot"},
		ProposedBy:    actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "treasury-agent"},
		Payload:       json.RawMessage(`{"asset":"USDC","amount":"25000.00","network":"ethereum","destination_address":"0x123"}`),
		Metadata:      map[string]any{"ticket_id": "CHG-1234"},
	}
}

func TestUsecasesCreate(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	action, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if action.ID.String() == "" {
		t.Fatal("expected action id")
	}
	if action.Status != actiondomain.ActionStatusPendingApproval {
		t.Fatalf("unexpected action status: %s", action.Status)
	}
	if action.Decision != actiondomain.DecisionRequireApproval {
		t.Fatalf("unexpected decision: %s", action.Decision)
	}
	if action.Approval == nil || action.Approval.Status != actiondomain.ApprovalStatusPending {
		t.Fatalf("unexpected approval: %#v", action.Approval)
	}
	if action.Risk.Level != actiondomain.RiskLevelHigh {
		t.Fatalf("unexpected risk: %#v", action.Risk)
	}
	if len(action.Evidence) == 0 {
		t.Fatal("expected evidence records")
	}
}

func TestUsecasesCreateRejectsUnsupportedActionType(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	req := validCreateRequest()
	req.ActionType = actiondomain.ActionType("rotate_key")
	req.Justification = "rotate signer key"
	req.RequestedBy = actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "ops-bot"}
	req.ProposedBy = actiondomain.ActorRef{Type: actiondomain.ActorTypeAgent, ID: "ops-agent"}
	req.Payload = json.RawMessage(`{}`)

	_, err := uc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestUsecasesCreateUsesResolvedResourceAndPolicy(t *testing.T) {
	t.Parallel()

	source := &stubPolicySource{
		items: []ActionPolicy{
			{
				ID:                 "policy_allow_review",
				ActionType:         "withdrawal",
				ResourceType:       "wallet",
				Effect:             "allow",
				Priority:           10,
				Expression:         `resource.environment == "prod"`,
				Reason:             "production wallet withdrawals need approval",
				RequireApproval:    true,
				ApprovalTTLSeconds: 600,
				Enabled:            true,
			},
		},
	}
	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:          "wallet_hot_usdc_1",
				Type:        actiondomain.ResourceTypeWallet,
				Name:        "wallet hot usdc 1",
				Environment: "prod",
				Chain:       "ethereum",
				Labels:      map[string]string{"tier": "hot"},
				Criticality: "critical",
			},
		}).
		WithPolicySource(source)

	action, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if source.gotActionType != "withdrawal" || source.gotResource != "wallet" {
		t.Fatalf("unexpected policy lookup: action_type=%q resource_type=%q", source.gotActionType, source.gotResource)
	}
	if action.Status != actiondomain.ActionStatusPendingApproval {
		t.Fatalf("unexpected action status: %s", action.Status)
	}
	if action.Decision != actiondomain.DecisionRequireApproval {
		t.Fatalf("unexpected decision: %s", action.Decision)
	}
	if action.Approval == nil {
		t.Fatal("expected approval to be present")
	}
	if got := action.Approval.ExpiresAt.Sub(action.CreatedAt); got != 10*time.Minute {
		t.Fatalf("unexpected approval ttl: %s", got)
	}
	if action.Risk.Score != 95 {
		t.Fatalf("unexpected risk score: %d", action.Risk.Score)
	}
	if len(action.Evidence) != 3 {
		t.Fatalf("unexpected evidence count: %d", len(action.Evidence))
	}
	if action.Evidence[1].Kind != "resource_resolution" || action.Evidence[2].Kind != "policy_decision" {
		t.Fatalf("unexpected evidence chain: %#v", action.Evidence)
	}
}

func TestUsecasesCreateCanBlockWithPolicy(t *testing.T) {
	t.Parallel()

	incidents := &stubIncidentSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:          "wallet_hot_usdc_1",
				Type:        actiondomain.ResourceTypeWallet,
				Environment: "prod",
				Criticality: "high",
			},
		}).
		WithPolicySource(&stubPolicySource{
			items: []ActionPolicy{
				{
					ID:           "policy_block_large_withdrawals",
					ActionType:   "withdrawal",
					ResourceType: "wallet",
					Effect:       "deny",
					Priority:     1,
					Expression:   `action.source_system == "treasury-orchestrator"`,
					Reason:       "blocked by policy",
					Enabled:      true,
				},
			},
		}).
		WithIncidentSink(incidents)

	action, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if action.Status != actiondomain.ActionStatusBlocked {
		t.Fatalf("unexpected action status: %s", action.Status)
	}
	if action.Decision != actiondomain.DecisionDeny {
		t.Fatalf("unexpected decision: %s", action.Decision)
	}
	if action.Approval != nil {
		t.Fatalf("did not expect approval on blocked action: %#v", action.Approval)
	}
	if len(incidents.items) != 1 {
		t.Fatalf("expected one incident, got %d", len(incidents.items))
	}
	if incidents.items[0].Trigger != IncidentTriggerBlockedAction || incidents.items[0].SourceID != action.ID.String() {
		t.Fatalf("unexpected incident request: %#v", incidents.items[0])
	}
}

func TestUsecasesCreateCanAllowWithoutApproval(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:          "wallet_hot_usdc_1",
				Type:        actiondomain.ResourceTypeWallet,
				Environment: "prod",
				Criticality: "medium",
			},
		}).
		WithPolicySource(&stubPolicySource{
			items: []ActionPolicy{
				{
					ID:           "policy_allow_auto",
					ActionType:   "withdrawal",
					ResourceType: "wallet",
					Effect:       "allow",
					Priority:     1,
					Expression:   `resource.environment == "prod"`,
					Reason:       "allow auto execution",
					Enabled:      true,
				},
			},
		})

	action, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if action.Status != actiondomain.ActionStatusApproved {
		t.Fatalf("unexpected action status: %s", action.Status)
	}
	if action.Decision != actiondomain.DecisionAllow {
		t.Fatalf("unexpected decision: %s", action.Decision)
	}
	if action.Approval != nil {
		t.Fatalf("did not expect approval: %#v", action.Approval)
	}
}

func TestUsecasesEmitsAuditLifecycle(t *testing.T) {
	t.Parallel()

	audits := &stubAuditSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithAuditSink(audits).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:          "wallet_hot_usdc_1",
				Type:        actiondomain.ResourceTypeWallet,
				Environment: "prod",
				Criticality: "medium",
			},
		}).
		WithPolicySource(&stubPolicySource{
			items: []ActionPolicy{
				{
					ID:                 "policy_allow_review",
					ActionType:         "withdrawal",
					ResourceType:       "wallet",
					Effect:             "allow",
					Priority:           1,
					Expression:         `resource.environment == "prod"`,
					Reason:             "allow with approval",
					RequireApproval:    true,
					ApprovalTTLSeconds: 600,
					Enabled:            true,
				},
			},
		}).
		WithExecutor(stubExecutor{
			result: map[string]any{
				"execution_id": "exec_01",
			},
		})

	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	approved, err := uc.Approve(context.Background(), created.ID, DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "alice"},
		Comment:   "approved",
	})
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	leased, err := uc.IssueLease(context.Background(), approved.ID)
	if err != nil {
		t.Fatalf("IssueLease returned error: %v", err)
	}
	_, err = uc.Execute(context.Background(), leased.ID, ExecuteRequest{
		LeaseID:    leased.Lease.ID,
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(audits.items) != 4 {
		t.Fatalf("unexpected audit count: %d", len(audits.items))
	}
	if audits.items[0].EventType != "action_created" ||
		audits.items[1].EventType != "action_approved" ||
		audits.items[2].EventType != "action_leased" ||
		audits.items[3].EventType != "action_executed" {
		t.Fatalf("unexpected audit lifecycle: %#v", audits.items)
	}
}

func TestUsecasesCreateRejectsMissingResolvedResource(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{err: ErrResourceNotFound})

	_, err := uc.Create(context.Background(), validCreateRequest())
	if err == nil {
		t.Fatal("expected resource resolution to fail")
	}

	var httpErr httpError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected httpError, got %T", err)
	}
	if httpErr.Status != 404 || httpErr.Code != "RESOURCE_NOT_FOUND" {
		t.Fatalf("unexpected error: %#v", httpErr)
	}
}

func TestUsecasesCreateBlockedIncidentFailureDoesNotBreakDecision(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:          "wallet_hot_usdc_1",
				Type:        actiondomain.ResourceTypeWallet,
				Environment: "prod",
				Criticality: "high",
			},
		}).
		WithPolicySource(&stubPolicySource{
			items: []ActionPolicy{
				{
					ID:           "policy_block_large_withdrawals",
					ActionType:   "withdrawal",
					ResourceType: "wallet",
					Effect:       "deny",
					Priority:     1,
					Expression:   `action.source_system == "treasury-orchestrator"`,
					Reason:       "blocked by policy",
					Enabled:      true,
				},
			},
		}).
		WithIncidentSink(&stubIncidentSink{err: errors.New("control-workers unavailable")})

	action, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if action.Status != actiondomain.ActionStatusBlocked {
		t.Fatalf("unexpected action status: %s", action.Status)
	}
}

func TestUsecasesRejectOpensIncident(t *testing.T) {
	t.Parallel()

	incidents := &stubIncidentSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).WithIncidentSink(incidents)

	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	rejected, err := uc.Reject(context.Background(), created.ID, DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "alice"},
		Comment:   "manual rejection after treasury review",
	})
	if err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	if rejected.Status != actiondomain.ActionStatusRejected {
		t.Fatalf("unexpected rejected status: %s", rejected.Status)
	}
	if len(incidents.items) != 1 {
		t.Fatalf("expected one incident, got %d", len(incidents.items))
	}
	if incidents.items[0].Trigger != IncidentTriggerApprovalRejected || incidents.items[0].Reason != "manual rejection after treasury review" {
		t.Fatalf("unexpected incident request: %#v", incidents.items[0])
	}
}

func TestUsecasesExecuteFailureOpensIncident(t *testing.T) {
	t.Parallel()

	incidents := &stubIncidentSink{}
	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithIncidentSink(incidents).
		WithExecutor(stubExecutor{err: errors.New("signer unreachable")})

	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	approved, err := uc.Approve(context.Background(), created.ID, DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "alice"},
		Comment:   "approved after treasury review",
	})
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	leased, err := uc.IssueLease(context.Background(), approved.ID)
	if err != nil {
		t.Fatalf("IssueLease returned error: %v", err)
	}

	_, err = uc.Execute(context.Background(), leased.ID, ExecuteRequest{
		LeaseID:    leased.Lease.ID,
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
	})
	if err == nil {
		t.Fatal("expected execute to fail")
	}
	var httpErr httpError
	if !errors.As(err, &httpErr) || httpErr.Code != "EXECUTION_FAILED" {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if len(incidents.items) != 1 {
		t.Fatalf("expected one incident, got %d", len(incidents.items))
	}
	last := incidents.items[len(incidents.items)-1]
	if last.Trigger != IncidentTriggerExecutionFailed || last.Reason != "signer unreachable" {
		t.Fatalf("unexpected incident request: %#v", last)
	}
}

func TestUsecasesCreateRejectsResourceTypeMismatchFromResolver(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil)).
		WithResourceResolver(stubResourceResolver{
			resource: actiondomain.ProtectedResource{
				ID:   "wallet_hot_usdc_1",
				Type: actiondomain.ResourceTypeVault,
			},
		})

	_, err := uc.Create(context.Background(), validCreateRequest())
	if err == nil {
		t.Fatal("expected resource_type mismatch")
	}

	var httpErr httpError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected httpError, got %T", err)
	}
	if httpErr.Status != 400 || httpErr.Code != "VALIDATION" {
		t.Fatalf("unexpected error: %#v", httpErr)
	}
}

func TestUsecasesApproveLeaseExecuteLifecycle(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))

	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	approved, err := uc.Approve(context.Background(), created.ID, DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "alice"},
		Comment:   "approved after treasury review",
	})
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if approved.Status != actiondomain.ActionStatusApproved || approved.Decision != actiondomain.DecisionAllow {
		t.Fatalf("unexpected approved action: %#v", approved)
	}

	leased, err := uc.IssueLease(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("IssueLease returned error: %v", err)
	}
	if leased.Status != actiondomain.ActionStatusLeased || leased.Lease == nil || leased.Lease.Status != actiondomain.LeaseStatusActive {
		t.Fatalf("unexpected leased action: %#v", leased)
	}

	executed, err := uc.Execute(context.Background(), created.ID, ExecuteRequest{
		LeaseID:    leased.Lease.ID,
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if executed.Status != actiondomain.ActionStatusExecuted || executed.Execution == nil || executed.Execution.Status != "success" {
		t.Fatalf("unexpected executed action: %#v", executed)
	}
	if executed.Lease == nil || executed.Lease.Status != actiondomain.LeaseStatusUsed {
		t.Fatalf("expected used lease after execution: %#v", executed.Lease)
	}
}

func TestUsecasesRejectBlocksLeaseIssuance(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	rejected, err := uc.Reject(context.Background(), created.ID, DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "bob"},
		Comment:   "destination not approved",
	})
	if err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	if rejected.Status != actiondomain.ActionStatusRejected || rejected.Decision != actiondomain.DecisionDeny {
		t.Fatalf("unexpected rejected action: %#v", rejected)
	}

	_, err = uc.IssueLease(context.Background(), created.ID)
	if err == nil {
		t.Fatal("expected lease issuance to fail")
	}
}

func TestUsecasesExecuteRejectsInvalidLease(t *testing.T) {
	t.Parallel()

	uc := NewUsecases(NewInMemoryRepository(nil))
	created, err := uc.Create(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := uc.Approve(context.Background(), created.ID, DecideRequest{DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeUser, ID: "alice"}}); err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	if _, err := uc.IssueLease(context.Background(), created.ID); err != nil {
		t.Fatalf("IssueLease returned error: %v", err)
	}

	_, err = uc.Execute(context.Background(), created.ID, ExecuteRequest{
		LeaseID:    uuid.New(),
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorTypeSystem, ID: "wallet-orchestrator"},
	})
	if err == nil {
		t.Fatal("expected execute to fail")
	}
}
