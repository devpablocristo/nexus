package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	p1, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("generate pkce: %v", err)
	}
	if p1.CodeVerifier == "" {
		t.Fatal("code verifier is empty")
	}
	if p1.CodeChallenge == "" {
		t.Fatal("code challenge is empty")
	}
	if p1.CodeVerifier == p1.CodeChallenge {
		t.Fatal("code verifier and challenge should differ")
	}

	// Two calls should produce different values.
	p2, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("generate pkce: %v", err)
	}
	if p1.CodeVerifier == p2.CodeVerifier {
		t.Fatal("two PKCE verifiers should not be equal")
	}
}

func TestGenerateState(t *testing.T) {
	s1, err := GenerateState()
	if err != nil {
		t.Fatalf("generate state: %v", err)
	}
	if s1 == "" {
		t.Fatal("state is empty")
	}
	s2, err := GenerateState()
	if err != nil {
		t.Fatalf("generate state: %v", err)
	}
	if s1 == s2 {
		t.Fatal("two states should not be equal")
	}
}

func TestGenerateNonce(t *testing.T) {
	n1, err := GenerateNonce()
	if err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	if n1 == "" {
		t.Fatal("nonce is empty")
	}
	n2, err := GenerateNonce()
	if err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	if n1 == n2 {
		t.Fatal("two nonces should not be equal")
	}
}

func TestTokenExchanger_AuthorizationURL(t *testing.T) {
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

	discovery := NewDiscoveryClient(srv.URL)
	exchanger := NewTokenExchanger(discovery, "my-client-id", "my-secret", "http://localhost:8080/callback")

	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("generate pkce: %v", err)
	}

	authURL, err := exchanger.AuthorizationURL(context.Background(), "test-state", "test-nonce", pkce, []string{"openid", "profile", "email"})
	if err != nil {
		t.Fatalf("authorization url: %v", err)
	}

	// Verify the URL contains expected parameters.
	if authURL == "" {
		t.Fatal("authorization URL is empty")
	}

	expectations := []string{
		"response_type=code",
		"client_id=my-client-id",
		"redirect_uri=",
		"state=test-state",
		"nonce=test-nonce",
		"code_challenge=",
		"code_challenge_method=S256",
		"scope=openid+profile+email",
	}

	for _, exp := range expectations {
		found := false
		if len(authURL) > 0 {
			for i := 0; i <= len(authURL)-len(exp); i++ {
				if authURL[i:i+len(exp)] == exp {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("expected authorization URL to contain %q, got: %s", exp, authURL)
		}
	}
}

func TestTokenExchanger_ExchangeCode_OK(t *testing.T) {
	tokenResp := TokenResponse{
		AccessToken: "access-token-123",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		IDToken:     "id-token-abc",
	}

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := DiscoveryDocument{
				Issuer:                "https://accounts.example.com",
				AuthorizationEndpoint: "https://accounts.example.com/authorize",
				TokenEndpoint:         "http://" + r.Host + "/token",
				JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(doc)
			return
		}
		if r.URL.Path == "/token" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.FormValue("grant_type") != "authorization_code" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			if r.FormValue("code") != "test-code" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_code"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tokenSrv.Close()

	discovery := NewDiscoveryClient(tokenSrv.URL)
	exchanger := NewTokenExchanger(discovery, "my-client-id", "my-secret", "http://localhost:8080/callback")

	got, err := exchanger.ExchangeCode(context.Background(), "test-code", "test-verifier")
	if err != nil {
		t.Fatalf("exchange code: %v", err)
	}
	if got.AccessToken != tokenResp.AccessToken {
		t.Fatalf("expected access_token %q, got %q", tokenResp.AccessToken, got.AccessToken)
	}
	if got.IDToken != tokenResp.IDToken {
		t.Fatalf("expected id_token %q, got %q", tokenResp.IDToken, got.IDToken)
	}
	if got.ExpiresIn != tokenResp.ExpiresIn {
		t.Fatalf("expected expires_in %d, got %d", tokenResp.ExpiresIn, got.ExpiresIn)
	}
}

func TestTokenExchanger_ExchangeCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := DiscoveryDocument{
				Issuer:                "https://accounts.example.com",
				AuthorizationEndpoint: "https://accounts.example.com/authorize",
				TokenEndpoint:         "http://" + r.Host + "/token",
				JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(doc)
			return
		}
		if r.URL.Path == "/token" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	discovery := NewDiscoveryClient(srv.URL)
	exchanger := NewTokenExchanger(discovery, "my-client-id", "my-secret", "http://localhost:8080/callback")

	_, err := exchanger.ExchangeCode(context.Background(), "bad-code", "test-verifier")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestTokenExchanger_ExchangeCode_MissingIDToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			doc := DiscoveryDocument{
				Issuer:                "https://accounts.example.com",
				AuthorizationEndpoint: "https://accounts.example.com/authorize",
				TokenEndpoint:         "http://" + r.Host + "/token",
				JWKSURI:               "https://accounts.example.com/.well-known/jwks.json",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(doc)
			return
		}
		if r.URL.Path == "/token" {
			resp := map[string]any{
				"access_token": "access-token-123",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	discovery := NewDiscoveryClient(srv.URL)
	exchanger := NewTokenExchanger(discovery, "my-client-id", "my-secret", "http://localhost:8080/callback")

	_, err := exchanger.ExchangeCode(context.Background(), "test-code", "test-verifier")
	if err == nil {
		t.Fatal("expected error for missing id_token")
	}
}
