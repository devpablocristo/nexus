package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeVerifier struct {
	claims map[string]any
	err    error
}

func (f fakeVerifier) VerifyToken(_ context.Context, _ string) (map[string]any, error) {
	return f.claims, f.err
}

func TestResolvePrincipal_OK(t *testing.T) {
	orgID := uuid.New()
	svc := NewService(fakeVerifier{
		claims: map[string]any{
			"iss":    "issuer",
			"aud":    []any{"nexus-core"},
			"org_id": orgID.String(),
			"sub":    "bot-1",
			"role":   "bot",
			"scopes": []any{"tools:run", "audit:read"},
		},
	}, Config{
		Issuer:      "issuer",
		Audience:    "nexus-core",
		OrgClaim:    "org_id",
		ActorClaim:  "sub",
		RoleClaim:   "role",
		ScopesClaim: "scopes",
	})

	got, err := svc.ResolvePrincipal(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OrgID != orgID {
		t.Fatalf("unexpected org id: %s", got.OrgID)
	}
	if got.Actor != "bot-1" || got.Role != "bot" {
		t.Fatalf("unexpected actor/role: %+v", got)
	}
	if len(got.Scopes) != 2 {
		t.Fatalf("unexpected scopes: %+v", got.Scopes)
	}
}

func TestResolvePrincipal_InvalidAudience(t *testing.T) {
	svc := NewService(fakeVerifier{
		claims: map[string]any{
			"iss":    "issuer",
			"aud":    "wrong",
			"org_id": uuid.NewString(),
		},
	}, Config{
		Issuer:   "issuer",
		Audience: "nexus-core",
		OrgClaim: "org_id",
	})

	if _, err := svc.ResolvePrincipal(context.Background(), "token"); err == nil {
		t.Fatal("expected error")
	}
}
