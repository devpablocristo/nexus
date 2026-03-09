package coreproxy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	opsaction "nexus-control-operators/internal/ops/actionengine"
	actiondomain "nexus-control-operators/internal/ops/actionengine/usecases/domain"
)

type CoreActionEngine struct {
	client  *Client
	mu      sync.Mutex
	store   map[uuid.UUID]opsaction.EngineRequest
	dataDir string
}

func NewCoreActionEngine(client *Client, dataDir string) *CoreActionEngine {
	e := &CoreActionEngine{
		client:  client,
		store:   map[uuid.UUID]opsaction.EngineRequest{},
		dataDir: dataDir,
	}
	e.loadStore()
	return e
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
	e.persistStore()
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
	actionReq.LeaseHeaders = mergeLeaseHeaders(actionReq.LeaseHeaders, req.LeaseHeaders)

	body := map[string]any{
		"action_type": actionReq.ActionType,
		"scope":       actionReq.Scope,
		"ttl_seconds": actionReq.TTLSeconds,
		"params":      actionReq.Params,
	}
	if actionReq.Scope == nil {
		body["scope"] = map[string]any{"level": "org", "org_id": orgID.String()}
	}
	ctx = WithExecutionLeaseHeaders(ctx, actionReq.LeaseHeaders)
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

	if req.ProposalID != nil {
		e.mu.Lock()
		delete(e.store, *req.ProposalID)
		e.persistStore()
		e.mu.Unlock()
	}

	return opsaction.EngineResult{
		Proposal:       proposal,
		IdempotencyKey: proposal.ID.String(),
	}, nil
}

func mergeLeaseHeaders(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range base {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	for k, v := range overlay {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (e *CoreActionEngine) Rollback(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	id := proposalIDOrNew(req.ProposalID)

	if req.ProposalID != nil {
		e.mu.Lock()
		delete(e.store, *req.ProposalID)
		e.persistStore()
		e.mu.Unlock()
	}

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

type proposalEntry struct {
	ID  string                  `json:"id"`
	Req opsaction.EngineRequest `json:"req"`
}

func (e *CoreActionEngine) loadStore() {
	if e.dataDir == "" {
		return
	}
	path := filepath.Join(e.dataDir, "proposals.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var entries []proposalEntry
	if json.Unmarshal(data, &entries) != nil {
		return
	}
	for _, entry := range entries {
		if id, err := uuid.Parse(entry.ID); err == nil {
			e.store[id] = entry.Req
		}
	}
}

func (e *CoreActionEngine) persistStore() {
	if e.dataDir == "" {
		return
	}
	_ = os.MkdirAll(e.dataDir, 0o755)
	entries := make([]proposalEntry, 0, len(e.store))
	for id, req := range e.store {
		entries = append(entries, proposalEntry{ID: id.String(), Req: req})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	tmp := filepath.Join(e.dataDir, "proposals.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(e.dataDir, "proposals.json"))
}
