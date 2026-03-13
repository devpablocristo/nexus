package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

const maxIdempotencyKeyLength = 255

type idempotencyRepository interface {
	Get(ctx context.Context, toolName, key string) (*gwdomain.IdempotencyRecord, error)
	CreateInProgress(ctx context.Context, rec gwdomain.IdempotencyRecord) (bool, error)
	MarkCompleted(ctx context.Context, toolName, key string, snapshot gwdomain.IdempotencyResponseSnapshot) error
	MarkFailed(ctx context.Context, toolName, key string, snapshot gwdomain.IdempotencyResponseSnapshot) error
}

type runHTTPError struct {
	Status      int
	Code        string
	Message     string
	Idempotency *gwdomain.IdempotencyMeta
}

func (e runHTTPError) Error() string {
	return e.Message
}

func newRunHTTPError(status int, code, message string, idem *gwdomain.IdempotencyMeta) error {
	return runHTTPError{
		Status:      status,
		Code:        code,
		Message:     message,
		Idempotency: idem,
	}
}

func toRunHTTPError(err error, idem *gwdomain.IdempotencyMeta) error {
	var httpErr runHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Idempotency == nil && idem != nil {
			httpErr.Idempotency = idem
		}
		return httpErr
	}

	switch {
	case errors.Is(err, ErrToolNotFound):
		return newRunHTTPError(http.StatusNotFound, "NOT_FOUND", "tool not found", idem)
	case errors.Is(err, ErrToolDisabled):
		return newRunHTTPError(http.StatusForbidden, "TOOL_DISABLED", "tool disabled", idem)
	case errors.Is(err, ErrUnsupportedToolKind):
		return newRunHTTPError(http.StatusBadRequest, "UNSUPPORTED_TOOL_KIND", "unsupported tool kind", idem)
	case errors.Is(err, ErrInputSchemaInvalid):
		return newRunHTTPError(http.StatusBadRequest, "INPUT_SCHEMA_INVALID", "input does not match schema", idem)
	case errors.Is(err, ErrOutputSchemaInvalid):
		return newRunHTTPError(http.StatusBadGateway, "OUTPUT_SCHEMA_INVALID", "output does not match schema", idem)
	case errors.Is(err, ErrTimeoutExceeded):
		return newRunHTTPError(http.StatusRequestTimeout, "TIMEOUT", "run timed out", idem)
	case errors.Is(err, ErrPolicyDecision):
		return newRunHTTPError(http.StatusInternalServerError, "POLICY_DECISION_ERROR", "policy decision failed", idem)
	default:
		return newRunHTTPError(http.StatusBadGateway, "UPSTREAM_ERROR", err.Error(), idem)
	}
}

func parseIdempotencyKey(raw string) *string {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > maxIdempotencyKeyLength {
		return nil
	}
	return &v
}

func writeIdempotencyHeader(w http.ResponseWriter, outcome gwdomain.IdempotencyOutcome) {
	if outcome == "" {
		return
	}
	w.Header().Set("X-Idempotency-Outcome", string(outcome))
}

func (u *Usecases) resolveIdempotency(ctx context.Context, st *runState) (*gwdomain.RunResponse, error) {
	if u.idempotency == nil || strings.EqualFold(st.tool.Method, http.MethodGet) {
		if st.idemMeta.Present {
			st.idemMeta.Outcome = gwdomain.IdempotencySkippedNotWrite
		}
		return nil, nil
	}
	if !st.idemMeta.Present {
		return nil, nil
	}

	st.idemMeta.Outcome = gwdomain.IdempotencyNew
	fp, err := buildRequestFingerprint(st.tool.Name, st.input, st.context)
	if err != nil {
		return nil, err
	}
	st.requestFingerprint = fp

	existing, err := u.idempotency.Get(ctx, st.tool.Name, st.idempotencyKey)
	if err != nil {
		return nil, newRunHTTPError(http.StatusInternalServerError, "IDEMPOTENCY_STORE_ERROR", "idempotency store read failed", &st.idemMeta)
	}
	if existing != nil {
		if existing.RequestFingerprint != st.requestFingerprint {
			st.idemMeta.Outcome = gwdomain.IdempotencyConflict
			return nil, newRunHTTPError(http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key used with different payload", &st.idemMeta)
		}
		switch existing.Status {
		case gwdomain.IdempotencyStatusCompleted:
			st.idemMeta.Outcome = gwdomain.IdempotencyReplay
			return &gwdomain.RunResponse{
				RequestID:   st.requestID,
				Decision:    existing.Response.Decision,
				ToolName:    st.tool.Name,
				Status:      existing.Response.Status,
				Reason:      existing.Response.Reason,
				Result:      existing.Response.Result,
				LatencyMS:   timeSinceMS(st.start),
				HTTPStatus:  existing.Response.HTTPStatus,
				IntentID:    existing.Response.IntentID,
				ApprovalID:  existing.Response.ApprovalID,
				Idempotency: st.idemMeta,
			}, nil
		case gwdomain.IdempotencyStatusFailed:
			st.idemMeta.Outcome = gwdomain.IdempotencyReplay
			if existing.Response.Status == gwdomain.RunStatusBlocked {
				return &gwdomain.RunResponse{
					RequestID:   st.requestID,
					Decision:    existing.Response.Decision,
					ToolName:    st.tool.Name,
					Status:      existing.Response.Status,
					Reason:      existing.Response.Reason,
					Result:      existing.Response.Result,
					LatencyMS:   timeSinceMS(st.start),
					HTTPStatus:  existing.Response.HTTPStatus,
					IntentID:    existing.Response.IntentID,
					ApprovalID:  existing.Response.ApprovalID,
					Idempotency: st.idemMeta,
				}, nil
			}

			code := existing.Response.ErrorCode
			if code == "" {
				code = "IDEMPOTENCY_REPLAY_ERROR"
			}
			msg := existing.Response.ErrorMsg
			if msg == "" {
				msg = "previous failed idempotent request"
			}
			status := existing.Response.HTTPStatus
			if status == 0 {
				status = http.StatusBadGateway
			}
			return nil, newRunHTTPError(status, code, msg, &st.idemMeta)
		case gwdomain.IdempotencyStatusInProgress:
			st.idemMeta.Outcome = gwdomain.IdempotencyInProgress
			return nil, newRunHTTPError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "idempotent request in progress", &st.idemMeta)
		}
	}

	inserted, err := u.idempotency.CreateInProgress(ctx, gwdomain.IdempotencyRecord{
		ToolName:           st.tool.Name,
		IdempotencyKey:     st.idempotencyKey,
		RequestFingerprint: st.requestFingerprint,
	})
	if err != nil {
		return nil, newRunHTTPError(http.StatusInternalServerError, "IDEMPOTENCY_STORE_ERROR", "idempotency store write failed", &st.idemMeta)
	}
	if !inserted {
		st.idemMeta.Outcome = gwdomain.IdempotencyInProgress
		return nil, newRunHTTPError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "idempotent request in progress", &st.idemMeta)
	}
	st.createdIdempotencyRow = true
	return nil, nil
}

func (u *Usecases) markCompletedIdempotency(ctx context.Context, st runState, resp gwdomain.RunResponse) {
	if !st.createdIdempotencyRow || u.idempotency == nil {
		return
	}
	_ = u.idempotency.MarkCompleted(ctx, st.tool.Name, st.idempotencyKey, snapshotFromRunResponse(resp))
}

func (u *Usecases) markFailedIdempotency(ctx context.Context, st runState, snapshot gwdomain.IdempotencyResponseSnapshot) {
	if !st.createdIdempotencyRow || u.idempotency == nil {
		return
	}
	_ = u.idempotency.MarkFailed(ctx, st.tool.Name, st.idempotencyKey, snapshot)
}

func buildRequestFingerprint(toolName string, input, contextMap map[string]any) (string, error) {
	inCanon, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	ctxCanon, err := json.Marshal(contextMap)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{toolName, string(inCanon), string(ctxCanon)}, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func snapshotFromRunResponse(resp gwdomain.RunResponse) gwdomain.IdempotencyResponseSnapshot {
	return gwdomain.IdempotencyResponseSnapshot{
		Decision:   resp.Decision,
		Status:     resp.Status,
		Reason:     resp.Reason,
		Result:     resp.Result,
		HTTPStatus: resp.HTTPStatus,
		IntentID:   resp.IntentID,
		ApprovalID: resp.ApprovalID,
	}
}

func snapshotFromRunError(err error) (gwdomain.IdempotencyResponseSnapshot, bool) {
	var httpErr runHTTPError
	if !errors.As(err, &httpErr) {
		return gwdomain.IdempotencyResponseSnapshot{}, false
	}
	return gwdomain.IdempotencyResponseSnapshot{
		Decision:   gwdomain.DecisionAllow,
		Status:     gwdomain.RunStatusError,
		ErrorCode:  httpErr.Code,
		ErrorMsg:   httpErr.Message,
		HTTPStatus: httpErr.Status,
	}, true
}

func timeSinceMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
