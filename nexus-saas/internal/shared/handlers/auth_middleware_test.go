package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-saas/cmd/config"
	"nexus-saas/internal/identity/usecases/domain"
	orgdomain "nexus-saas/internal/org/usecases/domain"
)

type fakeOrgAuth struct {
	principal orgdomain.Principal
	err       error
}

func (f fakeOrgAuth) ResolvePrincipal(_ context.Context, _ string) (orgdomain.Principal, error) {
	return f.principal, f.err
}

type fakeJWTAuth struct {
	principal domain.Principal
	err       error
}

func (f fakeJWTAuth) ResolvePrincipal(_ context.Context, _ string) (domain.Principal, error) {
	return f.principal, f.err
}

func TestAuthMiddleware_AllowsAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(zerolog.Nop(), config.ServiceConfig{
		AuthAllowAPIKey: true,
		AuthEnableJWT:   false,
	}, fakeOrgAuth{
		principal: orgdomain.Principal{OrgID: uuid.New(), Scopes: []string{"tools:run"}},
	}, fakeJWTAuth{}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderAPIKey, "plain-api-key")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
}

func TestAuthMiddleware_PrefersJWTWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	orgID := uuid.New()
	r.Use(AuthMiddleware(zerolog.Nop(), config.ServiceConfig{
		AuthAllowAPIKey: false,
		AuthEnableJWT:   true,
	}, fakeOrgAuth{}, fakeJWTAuth{
		principal: domain.Principal{OrgID: orgID, Actor: "bot", Role: "secops", Scopes: []string{"tools:run"}},
	}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
}

func TestAuthMiddleware_BlocksIPAfterRepeatedFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(zerolog.Nop(), config.ServiceConfig{
		AuthAllowAPIKey: true,
		AuthEnableJWT:   false,
	}, fakeOrgAuth{
		err: errors.New("invalid"),
	}, fakeJWTAuth{}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.77:12345"
		req.Header.Set(HeaderAPIKey, "bad-key")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401 got %d", i+1, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.77:12345"
	req.Header.Set(HeaderAPIKey, "bad-key")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after lockout, got %d", rr.Code)
	}
}
