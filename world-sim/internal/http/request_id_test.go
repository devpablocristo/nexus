package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestRequestIDFrom_HeaderPrecedence(t *testing.T) {
	req := httptest.NewRequest("POST", "/tools/world.move", nil)
	req.Header.Set("X-Nexus-Request-Id", "rid-header")

	if got := requestIDFrom(req, "rid-body"); got != "rid-header" {
		t.Fatalf("expected header request id precedence, got %q", got)
	}
}

func TestRequestIDFrom_FallbackBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/tools/world.move", nil)
	if got := requestIDFrom(req, "rid-body"); got != "rid-body" {
		t.Fatalf("expected body request id fallback, got %q", got)
	}
}
