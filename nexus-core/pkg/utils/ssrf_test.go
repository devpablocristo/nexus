package utils

import (
	"testing"
)

func TestValidateEgressURL_BlocksPrivateIPs(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/secret",
		"http://localhost/admin",
		"http://10.0.0.1/internal",
		"http://172.16.0.1/internal",
		"http://192.168.1.1/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/admin",
		"http://0.0.0.0/",
		"http://[fd12:3456:789a::1]/admin", // IPv6 ULA (fc00::/7)
	}
	for _, u := range blocked {
		if err := ValidateEgressURL(u); err == nil {
			t.Errorf("expected %s to be blocked, but was allowed", u)
		}
	}
}

func TestValidateEgressURL_BlocksNonHTTPSchemes(t *testing.T) {
	blocked := []string{
		"ftp://example.com/file",
		"gopher://example.com/",
		"file:///etc/passwd",
		"dict://example.com/",
		"",
	}
	for _, u := range blocked {
		if err := ValidateEgressURL(u); err == nil {
			t.Errorf("expected %s to be blocked, but was allowed", u)
		}
	}
}

func TestValidateEgressURL_AllowsPublicHTTPS(t *testing.T) {
	// Only test with IP literals to avoid DNS dependency in tests.
	allowed := []string{
		"https://8.8.8.8/api",
		"http://1.1.1.1/endpoint",
	}
	for _, u := range allowed {
		if err := ValidateEgressURL(u); err != nil {
			t.Errorf("expected %s to be allowed, got: %v", u, err)
		}
	}
}

func TestValidateEgressURLWithAllowlist_AllowsExactHostPort(t *testing.T) {
	if err := ValidateEgressURLWithAllowlist("http://sim-engine:8087/tools/world.observe", "sim-engine:8087"); err != nil {
		t.Fatalf("expected allowlisted host:port to pass, got: %v", err)
	}
}

func TestValidateEgressURLWithAllowlist_WildcardIsRejected(t *testing.T) {
	if err := ValidateEgressURLWithAllowlist("http://127.0.0.1/internal", "127.0.0.*:80"); err == nil {
		t.Fatalf("expected wildcard allowlist entry to be rejected")
	}
}

func TestValidateEgressURLWithAllowlist_NonAllowlistedPrivateStillBlocked(t *testing.T) {
	if err := ValidateEgressURLWithAllowlist("http://127.0.0.1/internal", "sim-engine:8087"); err == nil {
		t.Fatalf("expected non-allowlisted private destination to be blocked")
	}
}
