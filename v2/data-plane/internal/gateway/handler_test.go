package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"nexus/v2/data-plane/internal/egress"
	httpexec "nexus/v2/data-plane/internal/gateway/executor/http"
	"nexus/v2/data-plane/internal/gateway/executor/ratelimit"
	gwdto "nexus/v2/data-plane/internal/gateway/handler/dto"
	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
	"nexus/v2/data-plane/internal/policy"
	policydomain "nexus/v2/data-plane/internal/policy/usecases/domain"
	"nexus/v2/data-plane/internal/secrets"
	secretdomain "nexus/v2/data-plane/internal/secrets/usecases/domain"
	"nexus/v2/data-plane/internal/tool"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		requestBody       string
		policies          []policydomain.Policy
		upstreamDelay     time.Duration
		wantStatus        int
		wantDecision      string
		wantRunStatus     string
		wantReason        string
		wantErrorCode     string
		wantUpstreamCalls int32
		wantReceivedHello string
	}{
		{
			name:              "happy path default allow",
			requestBody:       `{"tool_name":"echo","input":{"hello":"world"}}`,
			wantStatus:        http.StatusOK,
			wantDecision:      "allow",
			wantRunStatus:     "success",
			wantUpstreamCalls: 1,
			wantReceivedHello: "world",
		},
		{
			name:              "happy path by tool_id",
			requestBody:       `{"tool_id":"tool_echo","input":{"hello":"world"}}`,
			wantStatus:        http.StatusOK,
			wantDecision:      "allow",
			wantRunStatus:     "success",
			wantUpstreamCalls: 1,
			wantReceivedHello: "world",
		},
		{
			name:        "deny policy blocks before upstream",
			requestBody: `{"tool_name":"echo","input":{"hello":"blocked"}}`,
			policies: []policydomain.Policy{
				{
					ToolName:   "echo",
					Effect:     policydomain.EffectDeny,
					Priority:   1,
					Expression: `input.hello == "blocked"`,
					Reason:     "blocked by policy",
					Enabled:    true,
				},
			},
			wantStatus:        http.StatusForbidden,
			wantDecision:      "deny",
			wantRunStatus:     "blocked",
			wantReason:        "blocked by policy",
			wantUpstreamCalls: 0,
		},
		{
			name:              "timeout_ms budget exceeded",
			requestBody:       `{"tool_name":"echo","timeout_ms":1000,"input":{"hello":"world"}}`,
			upstreamDelay:     1100 * time.Millisecond,
			wantStatus:        http.StatusRequestTimeout,
			wantErrorCode:     "TIMEOUT",
			wantUpstreamCalls: 1,
		},
		{
			name:              "timeout_ms below min clamps and still succeeds",
			requestBody:       `{"tool_name":"echo","timeout_ms":1,"input":{"hello":"world"}}`,
			upstreamDelay:     200 * time.Millisecond,
			wantStatus:        http.StatusOK,
			wantDecision:      "allow",
			wantRunStatus:     "success",
			wantUpstreamCalls: 1,
			wantReceivedHello: "world",
		},
		{
			name:        "invalid persisted policy fails as policy error",
			requestBody: `{"tool_name":"echo","input":{"hello":"world"}}`,
			policies: []policydomain.Policy{
				{
					ToolName:   "echo",
					Effect:     policydomain.EffectDeny,
					Priority:   1,
					Expression: `"blocked"`,
					Enabled:    true,
				},
			},
			wantStatus:        http.StatusInternalServerError,
			wantErrorCode:     "POLICY_DECISION_ERROR",
			wantUpstreamCalls: 0,
		},
		{
			name:          "missing tool identifier fails validation",
			requestBody:   `{"input":{"hello":"world"}}`,
			wantStatus:    http.StatusBadRequest,
			wantErrorCode: "VALIDATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var upstreamCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalls.Add(1)
				if r.Method != http.MethodPost {
					t.Fatalf("unexpected method: %s", r.Method)
				}
				if tt.upstreamDelay > 0 {
					time.Sleep(tt.upstreamDelay)
				}

				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"received": payload,
				})
			}))
			defer upstream.Close()

			tools := tool.NewInMemoryRepository([]tool.Definition{
				{
					ID:               "tool_echo",
					Name:             "echo",
					Kind:             tool.KindHTTP,
					Method:           http.MethodPost,
					URL:              upstream.URL,
					Enabled:          true,
					InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
					OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
				},
			})

			policies := policy.NewInMemoryRepository(tt.policies)
			usecase := NewUsecases(
				tools,
				policies,
				NewInMemoryIdempotencyRepository(),
				ratelimit.NewInMemoryLimiter(),
				newAllowedEgress(t, "tool_echo", upstream.URL),
				secrets.NewInMemoryRepository(nil),
				policy.NewEvaluator(),
				httpexec.NewExecutor(2*time.Second),
			)
			mux := http.NewServeMux()
			NewHandler(usecase).Register(mux)

			req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("unexpected status: got=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantErrorCode != "" {
				var resp gwdto.ErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if resp.Error.Code != tt.wantErrorCode {
					t.Fatalf("unexpected error code: got=%s want=%s", resp.Error.Code, tt.wantErrorCode)
				}
				if got := upstreamCalls.Load(); got != tt.wantUpstreamCalls {
					t.Fatalf("unexpected upstream calls: got=%d want=%d", got, tt.wantUpstreamCalls)
				}
				return
			}

			var resp gwdto.RunResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.Decision != tt.wantDecision {
				t.Fatalf("unexpected decision: got=%s want=%s", resp.Decision, tt.wantDecision)
			}
			if resp.Status != tt.wantRunStatus {
				t.Fatalf("unexpected run status: got=%s want=%s", resp.Status, tt.wantRunStatus)
			}
			if resp.Reason != tt.wantReason {
				t.Fatalf("unexpected reason: got=%q want=%q", resp.Reason, tt.wantReason)
			}
			if resp.ToolName != "echo" {
				t.Fatalf("unexpected tool name: %s", resp.ToolName)
			}
			if resp.RequestID == "" {
				t.Fatal("request_id should not be empty")
			}
			if got := upstreamCalls.Load(); got != tt.wantUpstreamCalls {
				t.Fatalf("unexpected upstream calls: got=%d want=%d", got, tt.wantUpstreamCalls)
			}

			if tt.wantReceivedHello == "" {
				if resp.Result != nil {
					t.Fatalf("expected nil result for blocked response, got=%#v", resp.Result)
				}
				return
			}

			result, ok := resp.Result.(map[string]any)
			if !ok {
				t.Fatalf("unexpected result type: %T", resp.Result)
			}
			received, ok := result["received"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected received type: %T", result["received"])
			}
			if received["hello"] != tt.wantReceivedHello {
				t.Fatalf("unexpected upstream payload: %#v", received)
			}
		})
	}
}

func TestRunIdempotencyReplayAndConflict(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"received": payload,
		})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	)
	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	firstRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "k1")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("unexpected first status: %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	var firstResp gwdto.RunResponse
	if err := json.NewDecoder(firstRec.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if firstResp.Idempotency == nil || firstResp.Idempotency.Outcome != "NEW" {
		t.Fatalf("unexpected first idempotency: %#v", firstResp.Idempotency)
	}
	if got := firstRec.Header().Get("X-Idempotency-Outcome"); got != "NEW" {
		t.Fatalf("unexpected first header: %q", got)
	}

	secondRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "k1")
	if secondRec.Code != http.StatusOK {
		t.Fatalf("unexpected replay status: %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	var secondResp gwdto.RunResponse
	if err := json.NewDecoder(secondRec.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if secondResp.Idempotency == nil || secondResp.Idempotency.Outcome != "REPLAY" {
		t.Fatalf("unexpected replay idempotency: %#v", secondResp.Idempotency)
	}
	if got := secondRec.Header().Get("X-Idempotency-Outcome"); got != "REPLAY" {
		t.Fatalf("unexpected replay header: %q", got)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("expected single upstream call after replay, got=%d", got)
	}

	conflictRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"other"}}`, "k1")
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("unexpected conflict status: %d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
	var conflictResp gwdto.ErrorResponse
	if err := json.NewDecoder(conflictRec.Body).Decode(&conflictResp); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if conflictResp.Error.Code != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("unexpected conflict code: %s", conflictResp.Error.Code)
	}
	if conflictResp.Idempotency == nil || conflictResp.Idempotency.Outcome != "CONFLICT" {
		t.Fatalf("unexpected conflict idempotency: %#v", conflictResp.Idempotency)
	}
	if got := conflictRec.Header().Get("X-Idempotency-Outcome"); got != "CONFLICT" {
		t.Fatalf("unexpected conflict header: %q", got)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("expected no extra upstream call on conflict, got=%d", got)
	}
}

func TestRunIdempotencyInProgress(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		<-release

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"received": payload,
		})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	)
	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "k2")
	}()

	for start := time.Now(); time.Since(start) < time.Second; {
		if upstreamCalls.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if upstreamCalls.Load() != 1 {
		t.Fatal("first request did not reach upstream")
	}

	secondRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "k2")
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("unexpected in-progress status: %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	var secondResp gwdto.ErrorResponse
	if err := json.NewDecoder(secondRec.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode in-progress response: %v", err)
	}
	if secondResp.Error.Code != "IDEMPOTENCY_IN_PROGRESS" {
		t.Fatalf("unexpected in-progress code: %s", secondResp.Error.Code)
	}
	if secondResp.Idempotency == nil || secondResp.Idempotency.Outcome != "IN_PROGRESS" {
		t.Fatalf("unexpected in-progress idempotency: %#v", secondResp.Idempotency)
	}

	close(release)
	firstRec := <-done
	if firstRec.Code != http.StatusOK {
		t.Fatalf("unexpected first completion status: %d body=%s", firstRec.Code, firstRec.Body.String())
	}
}

func doRunRequest(t *testing.T, handler http.Handler, body string, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRunRateLimitBlocksSecondRequest(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "world"}})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:                 "tool_echo",
			Name:               "echo",
			Kind:               tool.KindHTTP,
			Method:             http.MethodPost,
			URL:                upstream.URL,
			Enabled:            true,
			RateLimitPerMinute: 1,
			InputSchemaJSON:    []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON:   []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	)
	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	firstRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "")
	if firstRec.Code != http.StatusOK {
		t.Fatalf("unexpected first status: %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "")
	if secondRec.Code != http.StatusForbidden {
		t.Fatalf("unexpected second status: %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	var secondResp gwdto.RunResponse
	if err := json.NewDecoder(secondRec.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode blocked response: %v", err)
	}
	if secondResp.Status != "blocked" || secondResp.Decision != "deny" || secondResp.Reason != "rate limit exceeded" {
		t.Fatalf("unexpected rate-limit response: %#v", secondResp)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("unexpected upstream calls: %d", got)
	}
}

func TestRunEgressDeniedBlocksBeforeUpstream(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "world"}})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	denyingEgress := egress.NewUsecases(egress.NewInMemoryRepository(nil))
	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		denyingEgress,
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	)
	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	rec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"world"}}`, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var resp gwdto.RunResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode blocked response: %v", err)
	}
	if resp.Status != "blocked" || resp.Decision != "deny" || resp.Reason != "egress host denied" {
		t.Fatalf("unexpected egress response: %#v", resp)
	}
	if got := upstreamCalls.Load(); got != 0 {
		t.Fatalf("unexpected upstream calls: %d", got)
	}
}

func TestRunInjectsSecretHeaders(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotRequestID string
	var gotCustom string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotRequestID = r.Header.Get("X-Nexus-Request-Id")
		gotCustom = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "world"}})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	secretRepo := secrets.NewInMemoryRepository([]secretdomain.ToolSecret{
		{ToolID: "tool_echo", SecretType: "header", KeyName: "X-API-Key", PlaintextValue: "abc123", Enabled: true},
		{ToolID: "tool_echo", SecretType: "bearer", PlaintextValue: "token-xyz", Enabled: true},
	})
	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secretRepo,
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	)
	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBufferString(`{"request_id":"req-123","tool_name":"echo","input":{"hello":"world"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if gotCustom != "abc123" {
		t.Fatalf("unexpected custom header: %q", gotCustom)
	}
	if gotAuth != "Bearer token-xyz" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotRequestID != "req-123" {
		t.Fatalf("unexpected request id header: %q", gotRequestID)
	}
}

func TestRunApprovalRequiredCreatesIntentAndReplays(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})

	policies := policy.NewInMemoryRepository([]policydomain.Policy{
		{
			ToolName:           "echo",
			Effect:             policydomain.EffectAllow,
			Priority:           1,
			Expression:         `input.hello == "review"`,
			Reason:             "operator approval required",
			RequireApproval:    true,
			ApprovalTTLSeconds: 600,
			Enabled:            true,
		},
	})

	usecase := NewUsecases(
		tools,
		policies,
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	).WithIntentRepository(NewInMemoryIntentRepository()).WithApproval(newApprovalAdapter())

	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	firstRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"review"}}`, "approval-key")
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected first status: %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	if got := firstRec.Header().Get("X-Idempotency-Outcome"); got != "NEW" {
		t.Fatalf("unexpected first idempotency header: %q", got)
	}

	var firstResp gwdto.RunResponse
	if err := json.NewDecoder(firstRec.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if firstResp.Status != "blocked" || firstResp.Decision != "deny" {
		t.Fatalf("unexpected first response: %#v", firstResp)
	}
	if firstResp.IntentID == "" || firstResp.ApprovalID == "" {
		t.Fatalf("expected intent and approval ids: %#v", firstResp)
	}
	if got := upstreamCalls.Load(); got != 0 {
		t.Fatalf("approval should block before upstream, got calls=%d", got)
	}

	secondRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"review"}}`, "approval-key")
	if secondRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected replay status: %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if got := secondRec.Header().Get("X-Idempotency-Outcome"); got != "REPLAY" {
		t.Fatalf("unexpected replay idempotency header: %q", got)
	}

	var secondResp gwdto.RunResponse
	if err := json.NewDecoder(secondRec.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if secondResp.IntentID != firstResp.IntentID || secondResp.ApprovalID != firstResp.ApprovalID {
		t.Fatalf("expected replay to preserve ids: first=%#v second=%#v", firstResp, secondResp)
	}
	if got := upstreamCalls.Load(); got != 0 {
		t.Fatalf("approval replay should not hit upstream, got calls=%d", got)
	}
}

func TestIntentsEndpoints(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": map[string]any{"hello": "review"}})
	}))
	defer upstream.Close()

	tools := tool.NewInMemoryRepository([]tool.Definition{
		{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              upstream.URL,
			Enabled:          true,
			InputSchemaJSON:  []byte(`{"type":"object","required":["hello"],"properties":{"hello":{"type":"string"}},"additionalProperties":true}`),
			OutputSchemaJSON: []byte(`{"type":"object","required":["received"],"properties":{"received":{"type":"object"}},"additionalProperties":true}`),
		},
	})
	intentRepo := NewInMemoryIntentRepository()
	usecase := NewUsecases(
		tools,
		policy.NewInMemoryRepository([]policydomain.Policy{
			{
				ToolName:           "echo",
				Effect:             policydomain.EffectAllow,
				Priority:           1,
				Expression:         `input.hello == "review"`,
				Reason:             "operator approval required",
				RequireApproval:    true,
				ApprovalTTLSeconds: 600,
				Enabled:            true,
			},
		}),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		newAllowedEgress(t, "tool_echo", upstream.URL),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	).WithIntentRepository(intentRepo).WithApproval(newApprovalAdapter())

	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	runRec := doRunRequest(t, mux, `{"tool_name":"echo","input":{"hello":"review"}}`, "")
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("unexpected run status: %d body=%s", runRec.Code, runRec.Body.String())
	}

	var runResp gwdto.RunResponse
	if err := json.NewDecoder(runRec.Body).Decode(&runResp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if runResp.IntentID == "" {
		t.Fatalf("expected intent id: %#v", runResp)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents?limit=10", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp gwdto.ListIntentsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("unexpected intents count: %d", len(listResp.Items))
	}
	if listResp.Items[0].ID != runResp.IntentID {
		t.Fatalf("unexpected listed intent: %#v", listResp.Items[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+runResp.IntentID, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getRec.Code, getRec.Body.String())
	}

	var item gwdto.IntentItem
	if err := json.NewDecoder(getRec.Body).Decode(&item); err != nil {
		t.Fatalf("decode intent response: %v", err)
	}
	if item.ID != runResp.IntentID {
		t.Fatalf("unexpected intent item: %#v", item)
	}
	if item.ApprovalID == nil || *item.ApprovalID != runResp.ApprovalID {
		t.Fatalf("unexpected approval link: %#v", item)
	}
	if item.RiskClass != "mutate_non_prod" {
		t.Fatalf("unexpected risk class: %#v", item)
	}
	if item.PreflightStatus != "not_required" {
		t.Fatalf("unexpected preflight status: %#v", item)
	}

	preflightReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+runResp.IntentID+"/preflight", nil)
	preflightRec := httptest.NewRecorder()
	mux.ServeHTTP(preflightRec, preflightReq)
	if preflightRec.Code != http.StatusOK {
		t.Fatalf("unexpected preflight status: %d body=%s", preflightRec.Code, preflightRec.Body.String())
	}

	var preflightResp gwdto.PreflightReviewResponse
	if err := json.NewDecoder(preflightRec.Body).Decode(&preflightResp); err != nil {
		t.Fatalf("decode preflight response: %v", err)
	}
	if preflightResp.IntentID != runResp.IntentID {
		t.Fatalf("unexpected preflight item: %#v", preflightResp)
	}
	if preflightResp.Status != "not_required" {
		t.Fatalf("unexpected preflight review status: %#v", preflightResp)
	}
}

func TestIntentsEndpointsValidationAndNotFound(t *testing.T) {
	t.Parallel()

	usecase := NewUsecases(
		tool.NewInMemoryRepository(nil),
		policy.NewInMemoryRepository(nil),
		NewInMemoryIdempotencyRepository(),
		ratelimit.NewInMemoryLimiter(),
		egress.NewUsecases(egress.NewInMemoryRepository(nil)),
		secrets.NewInMemoryRepository(nil),
		policy.NewEvaluator(),
		httpexec.NewExecutor(2*time.Second),
	).WithIntentRepository(NewInMemoryIntentRepository()).WithLeaseRepository(NewInMemoryLeaseRepository())

	mux := http.NewServeMux()
	NewHandler(usecase).Register(mux)

	badLimitReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents?limit=0", nil)
	badLimitRec := httptest.NewRecorder()
	mux.ServeHTTP(badLimitRec, badLimitReq)
	if badLimitRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected bad limit status: %d body=%s", badLimitRec.Code, badLimitRec.Body.String())
	}

	var badLimitResp gwdto.ErrorResponse
	if err := json.NewDecoder(badLimitRec.Body).Decode(&badLimitResp); err != nil {
		t.Fatalf("decode bad limit response: %v", err)
	}
	if badLimitResp.Error.Code != "VALIDATION" {
		t.Fatalf("unexpected bad limit code: %#v", badLimitResp)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/not-a-uuid", nil)
	notFoundRec := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRec, notFoundReq)
	if notFoundRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected invalid id status: %d body=%s", notFoundRec.Code, notFoundRec.Body.String())
	}

	intentID := uuid.NewString()
	missingReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+intentID, nil)
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected missing intent status: %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	var missingResp gwdto.ErrorResponse
	if err := json.NewDecoder(missingRec.Body).Decode(&missingResp); err != nil {
		t.Fatalf("decode missing response: %v", err)
	}
	if missingResp.Error.Code != "NOT_FOUND" {
		t.Fatalf("unexpected missing code: %#v", missingResp)
	}

	preflightBadIDReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/not-a-uuid/preflight", nil)
	preflightBadIDRec := httptest.NewRecorder()
	mux.ServeHTTP(preflightBadIDRec, preflightBadIDReq)
	if preflightBadIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected invalid preflight id status: %d body=%s", preflightBadIDRec.Code, preflightBadIDRec.Body.String())
	}

	preflightMissingReq := httptest.NewRequest(http.MethodGet, "/v1/run/intents/"+intentID+"/preflight", nil)
	preflightMissingRec := httptest.NewRecorder()
	mux.ServeHTTP(preflightMissingRec, preflightMissingReq)
	if preflightMissingRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected missing preflight status: %d body=%s", preflightMissingRec.Code, preflightMissingRec.Body.String())
	}

	leaseBadIDReq := httptest.NewRequest(http.MethodPost, "/v1/run/intents/not-a-uuid/lease", nil)
	leaseBadIDRec := httptest.NewRecorder()
	mux.ServeHTTP(leaseBadIDRec, leaseBadIDReq)
	if leaseBadIDRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected invalid lease id status: %d body=%s", leaseBadIDRec.Code, leaseBadIDRec.Body.String())
	}

	leaseMissingReq := httptest.NewRequest(http.MethodPost, "/v1/run/intents/"+intentID+"/lease", nil)
	leaseMissingRec := httptest.NewRecorder()
	mux.ServeHTTP(leaseMissingRec, leaseMissingReq)
	if leaseMissingRec.Code != http.StatusNotFound {
		t.Fatalf("unexpected missing lease status: %d body=%s", leaseMissingRec.Code, leaseMissingRec.Body.String())
	}

	executeMissingLeaseReq := httptest.NewRequest(http.MethodPost, "/v1/run/intents/"+intentID+"/execute", bytes.NewBufferString(`{}`))
	executeMissingLeaseReq.Header.Set("Content-Type", "application/json")
	executeMissingLeaseRec := httptest.NewRecorder()
	mux.ServeHTTP(executeMissingLeaseRec, executeMissingLeaseReq)
	if executeMissingLeaseRec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected missing lease_id execute status: %d body=%s", executeMissingLeaseRec.Code, executeMissingLeaseRec.Body.String())
	}
}

func TestListIntentsMapsUnexpectedErrorToHTTPError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	NewHandler(handlerTestRunUsecase{listErr: errors.New("boom")}).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/run/intents", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: got=%d want=%d body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	var resp gwdto.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != "UPSTREAM_ERROR" {
		t.Fatalf("unexpected error code: %#v", resp)
	}
}

func newAllowedEgress(t *testing.T, toolID, rawURL string) *egress.Usecases {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return egress.NewUsecases(egress.NewInMemoryRepository([]egress.Rule{
		{ToolID: toolID, Host: parsed.Hostname(), Enabled: true},
	}))
}

func newApprovalAdapter() ApprovalPort {
	return approvalPortStub{}
}

type approvalPortStub struct{}

func (approvalPortStub) RequestApproval(_ context.Context, _ ApprovalRequest) (string, error) {
	return uuid.NewString(), nil
}

type handlerTestRunUsecase struct {
	listErr error
}

func (u handlerTestRunUsecase) Run(context.Context, gwdomain.RunRequest) (gwdomain.RunResponse, error) {
	return gwdomain.RunResponse{}, nil
}

func (u handlerTestRunUsecase) GetIntent(context.Context, uuid.UUID) (gwdomain.ExecutionIntent, error) {
	return gwdomain.ExecutionIntent{}, nil
}

func (u handlerTestRunUsecase) GetIntentPreflight(context.Context, uuid.UUID) (gwdomain.PreflightReview, error) {
	return gwdomain.PreflightReview{}, nil
}

func (u handlerTestRunUsecase) IssueExecutionLease(context.Context, uuid.UUID) (gwdomain.ExecutionLease, error) {
	return gwdomain.ExecutionLease{}, nil
}

func (u handlerTestRunUsecase) ListIntents(context.Context, int) ([]gwdomain.ExecutionIntent, error) {
	return nil, u.listErr
}

func (u handlerTestRunUsecase) ExecuteIntentWithLease(context.Context, uuid.UUID, uuid.UUID, int) (gwdomain.RunResponse, error) {
	return gwdomain.RunResponse{}, nil
}
