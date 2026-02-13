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
