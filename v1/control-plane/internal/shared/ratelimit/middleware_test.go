package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus/pkg/types"
)

func TestTenantLimiterBlocksAfterBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewTenantLimiter(1, 1)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(types.CtxKeyOrgID), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
		c.Next()
	})
	r.Use(limiter.Middleware())
	r.GET("/v1/admin/bootstrap", func(c *gin.Context) { c.Status(http.StatusOK) })

	req1 := httptest.NewRequest(http.MethodGet, "/v1/admin/bootstrap", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got=%d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/admin/bootstrap", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be rate limited, got=%d", w2.Code)
	}
	if got := w2.Header().Get("Retry-After"); got == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestTenantLimiterIsPerOrg(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewTenantLimiter(1, 1)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.Query("org") == "b" {
			c.Set(string(types.CtxKeyOrgID), uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"))
		} else {
			c.Set(string(types.CtxKeyOrgID), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
		}
		c.Next()
	})
	r.Use(limiter.Middleware())
	r.GET("/v1/admin/bootstrap", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/v1/admin/bootstrap?org=a", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("org a first request should pass, got=%d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/v1/admin/bootstrap?org=b", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("org b first request should pass, got=%d", w2.Code)
	}
}
