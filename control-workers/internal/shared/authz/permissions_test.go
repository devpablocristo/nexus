package authz

import "testing"

func TestHasAnyScope(t *testing.T) {
	scopes := []string{"read", "admin:console:write"}
	if !HasAnyScope(scopes, "admin:console:read", "admin:console:write") {
		t.Fatalf("expected scope match")
	}
}

func TestIsRole(t *testing.T) {
	role := "admin"
	if !IsRole(&role, "secops", "admin") {
		t.Fatalf("expected role match")
	}
}
