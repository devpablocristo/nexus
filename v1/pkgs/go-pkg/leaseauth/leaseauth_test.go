package leaseauth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignAndVerifyToken(t *testing.T) {
	now := time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC)
	token, err := SignToken("secret-key", Claims{
		OrgID:          "org-1",
		LeaseID:        "lease-1",
		IntentID:       "intent-1",
		ToolName:       "terraform-aws-apply",
		RiskClass:      "mutate_prod",
		CredentialMode: "aws_sts",
		Provider:       "aws",
		Scope:          "sts_assume_role",
		TargetEnv:      "prod",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nexus-core",
			Subject:   "execution_lease",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	claims, err := VerifyToken("secret-key", token, "nexus-core", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.LeaseID != "lease-1" {
		t.Fatalf("expected lease-1, got %q", claims.LeaseID)
	}
	if claims.Scope != "sts_assume_role" {
		t.Fatalf("expected sts_assume_role, got %q", claims.Scope)
	}
}

func TestVerifyTokenRejectsExpired(t *testing.T) {
	now := time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC)
	token, err := SignToken("secret-key", Claims{
		OrgID:          "org-1",
		LeaseID:        "lease-1",
		IntentID:       "intent-1",
		CredentialMode: "aws_sts",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nexus-core",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	_, err = VerifyToken("secret-key", token, "nexus-core", now.Add(2*time.Minute))
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired token error, got %v", err)
	}
}
