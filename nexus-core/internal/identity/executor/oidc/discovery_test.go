package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscoveryClient_Discover_OK(t *testing.T) {
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	got, err := client.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Issuer != doc.Issuer {
		t.Fatalf("expected issuer %q, got %q", doc.Issuer, got.Issuer)
	}
	if got.AuthorizationEndpoint != doc.AuthorizationEndpoint {
		t.Fatalf("expected authorization_endpoint %q, got %q", doc.AuthorizationEndpoint, got.AuthorizationEndpoint)
	}
	if got.TokenEndpoint != doc.TokenEndpoint {
		t.Fatalf("expected token_endpoint %q, got %q", doc.TokenEndpoint, got.TokenEndpoint)
	}
	if got.JWKSURI != doc.JWKSURI {
		t.Fatalf("expected jwks_uri %q, got %q", doc.JWKSURI, got.JWKSURI)
	}
}

func TestDiscoveryClient_Discover_Caching(t *testing.T) {
	callCount := 0
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	client.cacheTTL = 1 * time.Hour // Long TTL to ensure caching works.

	// First call should hit the server.
	_, err := client.Discover(context.Background())
	if err != nil {
		t.Fatalf("first discover: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call should use cache.
	_, err = client.Discover(context.Background())
	if err != nil {
		t.Fatalf("second discover: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (cached), got %d", callCount)
	}
}

func TestDiscoveryClient_Discover_CacheExpiry(t *testing.T) {
	callCount := 0
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	client.cacheTTL = 1 * time.Millisecond // Tiny TTL to force refresh.

	_, err := client.Discover(context.Background())
	if err != nil {
		t.Fatalf("first discover: %v", err)
	}

	// Wait for cache to expire.
	time.Sleep(5 * time.Millisecond)

	_, err = client.Discover(context.Background())
	if err != nil {
		t.Fatalf("second discover: %v", err)
	}
	if callCount < 2 {
		t.Fatalf("expected >= 2 calls after cache expiry, got %d", callCount)
	}
}

func TestDiscoveryClient_Discover_MissingIssuer(t *testing.T) {
	doc := DiscoveryDocument{
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	_, err := client.Discover(context.Background())
	if err == nil {
		t.Fatal("expected error for missing issuer")
	}
}

func TestDiscoveryClient_Discover_MissingJWKSURI(t *testing.T) {
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	_, err := client.Discover(context.Background())
	if err == nil {
		t.Fatal("expected error for missing jwks_uri")
	}
}

func TestDiscoveryClient_Discover_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	_, err := client.Discover(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestDiscoveryClient_Verifier_Lazy(t *testing.T) {
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)

	v1, err := client.Verifier(context.Background())
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	if v1 == nil {
		t.Fatal("expected non-nil verifier")
	}

	// Second call should return the same verifier instance.
	v2, err := client.Verifier(context.Background())
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	if v1 != v2 {
		t.Fatal("expected same verifier instance")
	}
}

func TestDiscoveryClient_JWKSURIChange_ResetsVerifier(t *testing.T) {
	jwksURI := "https://accounts.example.com/.well-known/jwks-v1.json"
	doc := DiscoveryDocument{
		Issuer:                "https://accounts.example.com",
		AuthorizationEndpoint: "https://accounts.example.com/authorize",
		TokenEndpoint:         "https://accounts.example.com/token",
		JWKSURI:               jwksURI,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()

	client := NewDiscoveryClient(srv.URL)
	client.cacheTTL = 1 * time.Millisecond

	v1, err := client.Verifier(context.Background())
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}

	// Change the JWKS URI.
	doc.JWKSURI = "https://accounts.example.com/.well-known/jwks-v2.json"

	// Wait for cache to expire.
	time.Sleep(5 * time.Millisecond)

	// Force a refresh by calling Discover.
	_, err = client.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	// The verifier should have been reset.
	v2, err := client.Verifier(context.Background())
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}

	if v1 == v2 {
		t.Fatal("expected different verifier instance after JWKS URI change")
	}
}
