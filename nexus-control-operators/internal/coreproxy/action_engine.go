package coreproxy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	opsaction "nexus-control-operators/internal/ops/actionengine"
	actiondomain "nexus-control-operators/internal/ops/actionengine/usecases/domain"
)

type CoreActionEngine struct {
	client *Client
	mu     sync.Mutex
	store  map[uuid.UUID]opsaction.EngineRequest
}

func NewCoreActionEngine(client *Client) *CoreActionEngine {
	return &CoreActionEngine{
		client: client,
		store:  map[uuid.UUID]opsaction.EngineRequest{},
	}
}

func (e *CoreActionEngine) DryRun(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	proposalID := deterministicProposalID(req)
	approvalRequired := req.ActionType == "quarantine_tenant"
	proposal := actiondomain.Proposal{
		ID:               proposalID,
		OrgID:            orgID,
		IncidentID:       req.IncidentID,
		ActionType:       req.ActionType,
		Scope:            req.Scope,
		Params:           req.Params,
		TTLSeconds:       req.TTLSeconds,
		EvidenceRefs:     req.EvidenceRefs,
		Status:           actiondomain.ProposalStatusDryRunOK,
		ApprovalRequired: approvalRequired,
		ProposedBy:       actor,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	e.mu.Lock()
	e.store[proposalID] = req
	e.mu.Unlock()
	return opsaction.EngineResult{
		Proposal:         proposal,
		IdempotencyKey:   proposalID.String(),
		ApprovalRequired: approvalRequired,
		Replay:           false,
	}, nil
}

func (e *CoreActionEngine) Apply(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	var actionReq opsaction.EngineRequest
	if req.ProposalID != nil {
		e.mu.Lock()
		actionReq = e.store[*req.ProposalID]
		e.mu.Unlock()
	} else {
		actionReq = req
	}

	body := map[string]any{
		"action_type": actionReq.ActionType,
		"scope":       actionReq.Scope,
		"ttl_seconds": actionReq.TTLSeconds,
		"params":      actionReq.Params,
	}
	if actionReq.Scope == nil {
		body["scope"] = map[string]any{"level": "org", "org_id": orgID.String()}
	}
	var resp map[string]any
	if err := e.client.DoJSON(ctx, "POST", "/internal/operators/actions/apply", body, &resp); err != nil {
		return opsaction.EngineResult{}, err
	}

	proposal := actiondomain.Proposal{
		ID:         proposalIDOrNew(req.ProposalID),
		OrgID:      orgID,
		ActionType: actionReq.ActionType,
		Scope:      actionReq.Scope,
		Params:     actionReq.Params,
		TTLSeconds: actionReq.TTLSeconds,
		Status:     actiondomain.ProposalStatusApplied,
		ProposedBy: actor,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	return opsaction.EngineResult{
		Proposal:       proposal,
		IdempotencyKey: proposal.ID.String(),
	}, nil
}

func (e *CoreActionEngine) Rollback(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	id := proposalIDOrNew(req.ProposalID)
	return opsaction.EngineResult{
		Proposal: actiondomain.Proposal{
			ID:         id,
			OrgID:      orgID,
			Status:     actiondomain.ProposalStatusRolledBack,
			ProposedBy: actor,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		},
		IdempotencyKey: id.String(),
	}, nil
}

// deterministicProposalID generates a reproducible UUID from the request
// so retries of the same event produce the same proposal, ensuring idempotency.
func deterministicProposalID(req opsaction.EngineRequest) uuid.UUID {
	incidentPart := ""
	if req.IncidentID != nil {
		incidentPart = req.IncidentID.String()
	}
	scopeRaw, _ := json.Marshal(req.Scope)
	seed := fmt.Sprintf("%s|%s|%x", incidentPart, req.ActionType, sha256.Sum256(scopeRaw))
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed))
}

func proposalIDOrNew(id *uuid.UUID) uuid.UUID {
	if id != nil {
		return *id
	}
	return uuid.New()
}
