package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"data-plane/cmd/config"
	"data-plane/internal/identity/usecases/domain"
	orgdomain "data-plane/internal/org/usecases/domain"
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
