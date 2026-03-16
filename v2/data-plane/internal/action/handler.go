package action

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	actiondto "nexus/v2/data-plane/internal/action/handler/dto"
	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type actionUsecase interface {
	Create(ctx context.Context, req CreateRequest) (actiondomain.Action, error)
	List(ctx context.Context, req ListRequest) ([]actiondomain.Action, error)
	GetByID(ctx context.Context, id uuid.UUID) (actiondomain.Action, error)
	GetRisk(ctx context.Context, id uuid.UUID) (actiondomain.RiskAssessment, error)
	GetEvidence(ctx context.Context, id uuid.UUID) ([]actiondomain.EvidenceRecord, error)
	Approve(ctx context.Context, id uuid.UUID, req DecideRequest) (actiondomain.Action, error)
	Reject(ctx context.Context, id uuid.UUID, req DecideRequest) (actiondomain.Action, error)
	IssueLease(ctx context.Context, id uuid.UUID) (actiondomain.Action, error)
	Execute(ctx context.Context, id uuid.UUID, req ExecuteRequest) (actiondomain.Action, error)
}

// Handler exposes the /v1/actions API.
type Handler struct {
	uc          actionUsecase
	idempotency IdempotencyStore
}

// NewHandler builds an action handler.
func NewHandler(uc actionUsecase) *Handler {
	return &Handler{uc: uc}
}

// WithIdempotency sets the idempotency store for the handler.
func (h *Handler) WithIdempotency(store IdempotencyStore) *Handler {
	h.idempotency = store
	return h
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/actions", h.create)
	mux.HandleFunc("GET /v1/actions", h.list)
	mux.HandleFunc("GET /v1/actions/{id}", h.getByID)
	mux.HandleFunc("GET /v1/actions/{id}/risk", h.getRisk)
	mux.HandleFunc("GET /v1/actions/{id}/evidence", h.getEvidence)
	mux.HandleFunc("POST /v1/actions/{id}/approve", h.approve)
	mux.HandleFunc("POST /v1/actions/{id}/reject", h.reject)
	mux.HandleFunc("POST /v1/actions/{id}/lease", h.issueLease)
	mux.HandleFunc("POST /v1/actions/{id}/execute", h.execute)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req actiondto.CreateActionRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeActionError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	// Idempotency check: if Idempotency-Key header is present and store is configured,
	// return cached response if the key was already used.
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" && h.idempotency != nil {
		entry, err := h.idempotency.Get(r.Context(), idempotencyKey)
		if err == nil && entry != nil {
			w.Header().Set("X-Idempotency-Replay", "true")
			w.Header().Set("Location", "/v1/actions/"+entry.ActionID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(entry.StatusCode)
			_, _ = w.Write(entry.Response)
			return
		}
	}

	created, err := h.uc.Create(r.Context(), CreateRequest{
		ActionType:    actiondomain.ActionType(req.ActionType),
		ResourceID:    req.ResourceID,
		ResourceType:  actiondomain.ResourceType(req.ResourceType),
		SourceSystem:  req.SourceSystem,
		Justification: req.Justification,
		RequestedBy: actiondomain.ActorRef{
			Type: actiondomain.ActorType(req.RequestedBy.Type),
			ID:   req.RequestedBy.ID,
		},
		ProposedBy: actiondomain.ActorRef{
			Type: actiondomain.ActorType(req.ProposedBy.Type),
			ID:   req.ProposedBy.ID,
		},
		Payload:  req.Payload,
		Metadata: req.Metadata,
	})
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	resp := toActionResponse(created)
	statusCode := http.StatusCreated

	// Store idempotency entry for successful creation.
	if idempotencyKey != "" && h.idempotency != nil {
		if respBytes, marshalErr := json.Marshal(resp); marshalErr == nil {
			_ = h.idempotency.Set(r.Context(), idempotencyKey, IdempotencyEntry{
				ActionID:   created.ID.String(),
				StatusCode: statusCode,
				Response:   respBytes,
				ExpiresAt:  time.Now().UTC().Add(defaultIdempotencyTTL),
			})
		}
	}

	w.Header().Set("Location", "/v1/actions/"+created.ID.String())
	sharedhandlers.WriteJSON(w, statusCode, resp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := sharedhandlers.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeActionError(w, http.StatusBadRequest, "VALIDATION", "limit must be a positive integer")
		return
	}

	items, err := h.uc.List(r.Context(), ListRequest{
		ActionType: r.URL.Query().Get("action_type"),
		Status:     r.URL.Query().Get("status"),
		Limit:      limit,
	})
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	resp := actiondto.ListActionsResponse{Items: make([]actiondto.ActionResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toActionResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	item, err := h.uc.GetByID(r.Context(), id)
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toActionResponse(item))
}

func (h *Handler) getRisk(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	risk, err := h.uc.GetRisk(r.Context(), id)
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, toRiskResponse(risk))
}

func (h *Handler) getEvidence(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	items, err := h.uc.GetEvidence(r.Context(), id)
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	resp := actiondto.EvidenceListResponse{Items: make([]actiondto.EvidenceRecordResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toEvidenceRecordResponse(item))
	}
	sharedhandlers.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, true)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, false)
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	var req actiondto.DecideActionRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeActionError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	decideReq := DecideRequest{
		DecidedBy: actiondomain.ActorRef{Type: actiondomain.ActorType(req.DecidedBy.Type), ID: req.DecidedBy.ID},
		Comment:   req.Comment,
	}

	var (
		item actiondomain.Action
		err  error
	)
	if approve {
		item, err = h.uc.Approve(r.Context(), id, decideReq)
	} else {
		item, err = h.uc.Reject(r.Context(), id, decideReq)
	}
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, toActionResponse(item))
}

func (h *Handler) issueLease(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	item, err := h.uc.IssueLease(r.Context(), id)
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, toActionResponse(item))
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionID(w, r)
	if !ok {
		return
	}

	var req actiondto.ExecuteActionRequest
	if err := sharedhandlers.DecodeJSON(r, &req); err != nil {
		writeActionError(w, http.StatusBadRequest, "INVALID_JSON", "invalid json")
		return
	}

	leaseID, err := uuid.Parse(req.LeaseID)
	if err != nil {
		writeActionError(w, http.StatusBadRequest, "VALIDATION", "invalid lease_id")
		return
	}

	item, err := h.uc.Execute(r.Context(), id, ExecuteRequest{
		LeaseID:    leaseID,
		ExecutedBy: actiondomain.ActorRef{Type: actiondomain.ActorType(req.ExecutedBy.Type), ID: req.ExecutedBy.ID},
	})
	if err != nil {
		writeActionUsecaseError(w, err)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, toActionResponse(item))
}

func parseActionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeActionError(w, http.StatusBadRequest, "VALIDATION", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeActionUsecaseError(w http.ResponseWriter, err error) {
	var httpErr httpError
	if errors.As(err, &httpErr) {
		writeActionError(w, httpErr.Status, httpErr.Code, httpErr.Message)
		return
	}
	writeActionError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
}

func writeActionError(w http.ResponseWriter, status int, code, message string) {
	sharedhandlers.WriteJSON(w, status, actiondto.ErrorResponse{
		Error: actiondto.ErrorObject{Code: code, Message: message},
	})
}

func toActionResponse(item actiondomain.Action) actiondto.ActionResponse {
	return actiondto.ActionResponse{
		ID:              item.ID.String(),
		ActionType:      string(item.Type),
		Status:          string(item.Status),
		Decision:        string(item.Decision),
		ResourceID:      item.ResourceID,
		ResourceType:    string(item.ResourceType),
		SourceSystem:    item.SourceSystem,
		Justification:   item.Justification,
		RequestedBy:     toActorDTO(item.RequestedBy),
		ProposedBy:      toActorDTO(item.ProposedBy),
		Payload:         item.Payload,
		Metadata:        cloneMap(item.Metadata),
		Risk:            toRiskResponse(item.Risk),
		EvidenceSummary: summarizeEvidence(item.Evidence),
		Approval:        toApprovalResponse(item.Approval),
		Lease:           toLeaseResponse(item.Lease),
		Execution:       toExecutionResponse(item.Execution),
		ExpiresAt:       item.ExpiresAt,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func toRiskResponse(item actiondomain.RiskAssessment) actiondto.RiskResponse {
	resp := actiondto.RiskResponse{
		Level:               string(item.Level),
		Score:               item.Score,
		Summary:             item.Summary,
		Profile:             actiondto.RiskProfileRefResponse{Name: item.Profile.Name, Version: item.Profile.Version},
		RiskPressure:        item.RiskPressure,
		SafetyPressure:      item.SafetyPressure,
		RawScore:            item.RawScore,
		DecisionScore:       item.DecisionScore,
		RecommendedDecision: string(item.RecommendedDecision),
		Factors:             make([]actiondto.RiskFactorResponse, 0, len(item.Factors)),
		Amplifications:      make([]actiondto.RiskInteractionResponse, 0, len(item.Amplifications)),
		Attenuations:        make([]actiondto.RiskInteractionResponse, 0, len(item.Attenuations)),
	}
	for _, factor := range item.Factors {
		resp.Factors = append(resp.Factors, actiondto.RiskFactorResponse{
			Code:            factor.Code,
			Type:            string(factor.Type),
			Active:          factor.Active,
			Weight:          factor.Weight,
			AppliedWeight:   factor.AppliedWeight,
			Summary:         factor.Summary,
			EvidenceQuality: string(factor.EvidenceQuality),
		})
	}
	for _, amplification := range item.Amplifications {
		resp.Amplifications = append(resp.Amplifications, actiondto.RiskInteractionResponse{
			Factors:    append([]string(nil), amplification.Factors...),
			Multiplier: amplification.Multiplier,
			Summary:    amplification.Summary,
		})
	}
	for _, attenuation := range item.Attenuations {
		resp.Attenuations = append(resp.Attenuations, actiondto.RiskInteractionResponse{
			Factors:    append([]string(nil), attenuation.Factors...),
			Multiplier: attenuation.Multiplier,
			Summary:    attenuation.Summary,
		})
	}
	return resp
}

func summarizeEvidence(items []actiondomain.EvidenceRecord) actiondto.EvidenceSummary {
	summary := actiondto.EvidenceSummary{Status: string(actiondomain.EvidenceStatusPassed), ChecksTotal: len(items)}
	for _, item := range items {
		switch item.Status {
		case actiondomain.EvidenceStatusPassed:
			summary.ChecksPassed++
		case actiondomain.EvidenceStatusFailed:
			summary.ChecksFailed++
			summary.Status = string(actiondomain.EvidenceStatusFailed)
		}
	}
	return summary
}

func toApprovalResponse(item *actiondomain.Approval) *actiondto.ApprovalResponse {
	if item == nil {
		return nil
	}

	var approvalID *string
	if item.ID != uuid.Nil {
		value := item.ID.String()
		approvalID = &value
	}

	resp := &actiondto.ApprovalResponse{
		Required:      true,
		ApprovalID:    approvalID,
		Status:        string(item.Status),
		RequiredCount: item.RequiredCount,
		GrantedCount:  item.GrantedCount,
		Comment:       item.Comment,
		ExpiresAt:     item.ExpiresAt,
		DecidedAt:     item.DecidedAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
	if item.DecidedBy != nil {
		actor := toActorDTO(*item.DecidedBy)
		resp.DecidedBy = &actor
	}
	return resp
}

func toLeaseResponse(item *actiondomain.ExecutionLease) *actiondto.LeaseResponse {
	if item == nil {
		return nil
	}
	return &actiondto.LeaseResponse{
		ID:     item.ID.String(),
		Status: string(item.Status),
		Scope: actiondto.LeaseScope{
			ActionID:     item.Scope.ActionID.String(),
			ActionType:   string(item.Scope.ActionType),
			ResourceID:   item.Scope.ResourceID,
			ResourceType: string(item.Scope.ResourceType),
		},
		ExpiresAt: item.ExpiresAt,
		UsedAt:    item.UsedAt,
		CreatedAt: item.CreatedAt,
	}
}

func toExecutionResponse(item *actiondomain.ExecutionResult) *actiondto.ExecutionResponse {
	if item == nil {
		return nil
	}
	return &actiondto.ExecutionResponse{
		Status:     item.Status,
		ExecutedBy: toActorDTO(item.ExecutedBy),
		Result:     cloneMap(item.Result),
		ExecutedAt: item.ExecutedAt,
	}
}

func toEvidenceRecordResponse(item actiondomain.EvidenceRecord) actiondto.EvidenceRecordResponse {
	return actiondto.EvidenceRecordResponse{
		ID:        item.ID.String(),
		ActionID:  item.ActionID.String(),
		Kind:      item.Kind,
		Status:    string(item.Status),
		Summary:   item.Summary,
		Details:   cloneMap(item.Details),
		CreatedAt: item.CreatedAt,
	}
}

func toActorDTO(actor actiondomain.ActorRef) actiondto.ActorRef {
	return actiondto.ActorRef{Type: string(actor.Type), ID: actor.ID}
}
