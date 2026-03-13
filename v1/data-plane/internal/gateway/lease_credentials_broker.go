package gateway

import (
	"context"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	tooldomain "data-plane/internal/tool/usecases/domain"
	"nexus/pkg/leaseauth"
)

// LeaseMetadataBroker materializa el lease como headers efimeros para operadores
// downstream. Con esto el runtime deja de depender de secretos estaticos cuando
// una ejecucion ya fue promovida al flujo controlado por lease.
type LeaseMetadataBroker struct {
	signingKey string
	issuer     string
}

func NewLeaseMetadataBroker(cfg Config) *LeaseMetadataBroker {
	issuer := strings.TrimSpace(cfg.LeaseTokenIssuer)
	if issuer == "" {
		issuer = "nexus-core"
	}
	return &LeaseMetadataBroker{
		signingKey: strings.TrimSpace(cfg.LeaseTokenSigningKey),
		issuer:     issuer,
	}
}

func (b *LeaseMetadataBroker) Issue(_ context.Context, _ uuid.UUID, tool tooldomain.Tool, lease gwdomain.ExecutionLease) (LeaseCredentialMaterial, error) {
	now := time.Now().UTC()
	token, err := leaseauth.SignToken(b.signingKey, leaseauth.Claims{
		OrgID:          lease.OrgID.String(),
		LeaseID:        lease.ID.String(),
		IntentID:       lease.IntentID.String(),
		ToolName:       tool.Name,
		RiskClass:      string(lease.RiskClass),
		CredentialMode: lease.CredentialMode,
		Provider:       strings.TrimSpace(anyToString(lease.CredentialHints["provider"])),
		Scope:          strings.TrimSpace(anyToString(lease.CredentialHints["scope"])),
		TargetEnv:      strings.TrimSpace(anyToString(lease.CredentialHints["target_env"])),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    b.issuer,
			Subject:   "execution_lease",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(lease.ExpiresAt.UTC()),
		},
	})
	if err != nil {
		return LeaseCredentialMaterial{}, err
	}
	headers := map[string]string{
		"X-Nexus-Lease-Id":        lease.ID.String(),
		"X-Nexus-Intent-Id":       lease.IntentID.String(),
		"X-Nexus-Credential-Mode": lease.CredentialMode,
		"X-Nexus-Risk-Class":      string(lease.RiskClass),
		"X-Nexus-Tool-Name":       tool.Name,
		"X-Nexus-Execution-Token": token,
	}
	if scope := strings.TrimSpace(anyToString(lease.CredentialHints["scope"])); scope != "" {
		headers["X-Nexus-Credential-Scope"] = scope
	}
	if provider := strings.TrimSpace(anyToString(lease.CredentialHints["provider"])); provider != "" {
		headers["X-Nexus-Credential-Provider"] = provider
	}
	if targetEnv := strings.TrimSpace(anyToString(lease.CredentialHints["target_env"])); targetEnv != "" {
		headers["X-Nexus-Target-Env"] = targetEnv
	}
	if shouldReplaceStaticHeaders(lease.CredentialMode) {
		headers["Authorization"] = "Bearer " + token
	}
	return LeaseCredentialMaterial{
		Headers:        headers,
		ReplaceHeaders: shouldReplaceStaticHeaders(lease.CredentialMode),
	}, nil
}

func shouldReplaceStaticHeaders(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "aws_sts", "kubeconfig_ephemeral", "session_bound":
		return true
	default:
		return false
	}
}
