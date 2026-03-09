package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-core/internal/dlp"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/policy"
	secretdomain "nexus-core/internal/secrets/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus/pkg/leaseauth"
	"nexus/pkg/validations/jsonschema"
)

type staticSecretRepo struct {
	secrets []secretdomain.ToolSecret
}

func (r staticSecretRepo) ListForTool(context.Context, uuid.UUID, uuid.UUID) ([]secretdomain.ToolSecret, error) {
	return r.secrets, nil
}

func TestRun_LeaseCredentialsReplaceStaticAuth(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	exec := &captureExecutor{}
	service := NewUsecases(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              toolID,
			OrgID:           orgID,
			Name:            "terraform-aws-apply",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "GET",
			URL:             "http://mock-tools:8081/tools/apply",
			InputSchemaJSON: []byte(`{"type":"object"}`),
			ActionType:      tooldomain.ActionRead,
			Enabled:         true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		staticSecretRepo{secrets: []secretdomain.ToolSecret{
			{ToolID: toolID, SecretType: "bearer", PlaintextValue: "static-token", Enabled: true},
			{ToolID: toolID, SecretType: "header", KeyName: "X-Static-Header", PlaintextValue: "static-value", Enabled: true},
		}},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{},
		nil,
		nil,
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		defaultConfig(),
		zerolog.Nop(),
	).WithLeaseCredentialBroker(NewLeaseMetadataBroker(defaultConfig()))

	resp, err := service.Run(context.Background(), orgID, gwdomain.RunRequest{
		ToolName: "terraform-aws-apply",
		Input:    map[string]any{"environment": "prod"},
		ExecutionLease: &gwdomain.ExecutionLease{
			ID:             uuid.New(),
			OrgID:          orgID,
			IntentID:       uuid.New(),
			ToolName:       "terraform-aws-apply",
			RiskClass:      gwdomain.RiskClassMutateProd,
			CredentialMode: "aws_sts",
			CredentialHints: map[string]any{
				"provider":   "aws",
				"scope":      "sts_assume_role",
				"target_env": "prod",
			},
			ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	authz := exec.lastHeaders["Authorization"]
	if !strings.HasPrefix(authz, "Bearer ") {
		t.Fatalf("expected bearer authorization header, got %q", authz)
	}
	if got := exec.lastHeaders["X-Static-Header"]; got != "" {
		t.Fatalf("expected static custom header to be replaced, got %q", got)
	}
	if got := exec.lastHeaders["X-Nexus-Credential-Mode"]; got != "aws_sts" {
		t.Fatalf("expected lease credential mode header, got %q", got)
	}
	if got := exec.lastHeaders["X-Nexus-Credential-Scope"]; got != "sts_assume_role" {
		t.Fatalf("expected lease credential scope header, got %q", got)
	}
	claims, err := leaseauth.VerifyToken(defaultConfig().LeaseTokenSigningKey, strings.TrimPrefix(authz, "Bearer "), defaultConfig().LeaseTokenIssuer, time.Now().UTC())
	if err != nil {
		t.Fatalf("verify execution token: %v", err)
	}
	if claims.Scope != "sts_assume_role" {
		t.Fatalf("expected sts_assume_role in token, got %q", claims.Scope)
	}
}

func TestRun_LeaseOnlyKeepsStaticSecrets(t *testing.T) {
	orgID := uuid.New()
	toolID := uuid.New()
	exec := &captureExecutor{}
	service := NewUsecases(
		fakeToolRepo{tool: tooldomain.Tool{
			ID:              toolID,
			OrgID:           orgID,
			Name:            "generic-http",
			Kind:            tooldomain.ToolKindHTTP,
			Method:          "GET",
			URL:             "http://mock-tools:8081/tools/generic",
			InputSchemaJSON: []byte(`{"type":"object"}`),
			ActionType:      tooldomain.ActionRead,
			Enabled:         true,
		}},
		fakePolicyRepo{},
		fakeAuditRepo{},
		staticSecretRepo{secrets: []secretdomain.ToolSecret{
			{ToolID: toolID, SecretType: "bearer", PlaintextValue: "static-token", Enabled: true},
		}},
		fakeEgress{},
		fakeLimiter{},
		exec,
		fakeIdempotency{},
		nil,
		nil,
		nil,
		fakeMetrics{},
		jsonschema.NewCompilerCache(),
		policy.NewEvaluator(),
		dlp.NewDetector(),
		defaultConfig(),
		zerolog.Nop(),
	).WithLeaseCredentialBroker(NewLeaseMetadataBroker(defaultConfig()))

	resp, err := service.Run(context.Background(), orgID, gwdomain.RunRequest{
		ToolName: "generic-http",
		Input:    map[string]any{"hello": "world"},
		ExecutionLease: &gwdomain.ExecutionLease{
			ID:             uuid.New(),
			OrgID:          orgID,
			IntentID:       uuid.New(),
			ToolName:       "generic-http",
			RiskClass:      gwdomain.RiskClassMutateNonProd,
			CredentialMode: "lease_only",
			CredentialHints: map[string]any{
				"provider": "generic",
				"scope":    "lease_only",
			},
			ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertSuccess(t, resp)
	if got := exec.lastHeaders["Authorization"]; got != "Bearer static-token" {
		t.Fatalf("expected static authorization header to be preserved, got %q", got)
	}
	if got := exec.lastHeaders["X-Nexus-Credential-Mode"]; got != "lease_only" {
		t.Fatalf("expected lease metadata header, got %q", got)
	}
	if got := exec.lastHeaders["X-Nexus-Execution-Token"]; got == "" {
		t.Fatalf("expected execution token header")
	}
}
