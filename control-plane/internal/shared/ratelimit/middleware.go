package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"nexus/pkg/types"
)

const (
	defaultRPS       = 100.0
	defaultBurst     = 200
	cleanupEvery     = 2 * time.Minute
	bucketTTL        = 10 * time.Minute
	retryAfterHeader = "1"
)

type tenantBucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// TenantLimiter applies per-tenant rate-limiting for authenticated /v1 SaaS endpoints.
type TenantLimiter struct {
	limiters sync.Map // map[string]*tenantBucket
	rps      float64
	burst    float64
	now      func() time.Time
}

// NewTenantLimiter returns a token-bucket limiter keyed by org_id.
func NewTenantLimiter(rps float64, burst int) *TenantLimiter {
	if rps <= 0 {
		rps = defaultRPS
	}
	if burst <= 0 {
		burst = defaultBurst
	}
	tl := &TenantLimiter{
		rps:   rps,
		burst: float64(burst),
		now:   time.Now,
	}
	go tl.cleanup()
	return tl
}

// Middleware enforces per-org requests/second limits.
func (tl *TenantLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID, ok := c.Get(string(types.CtxKeyOrgID))
		if !ok {
			c.Next()
			return
		}
		key := orgKey(orgID)
		if key == "" {
			c.Next()
			return
		}
		if !tl.allow(key) {
			c.Header("Retry-After", retryAfterHeader)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
			return
		}
		c.Next()
	}
}

func (tl *TenantLimiter) allow(key string) bool {
	now := tl.now().UTC()
	raw, _ := tl.limiters.LoadOrStore(key, &tenantBucket{
		tokens:     tl.burst,
		lastRefill: now,
		lastSeen:   now,
	})
	bucket := raw.(*tenantBucket)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * tl.rps
		if bucket.tokens > tl.burst {
			bucket.tokens = tl.burst
		}
		bucket.lastRefill = now
	}
	bucket.lastSeen = now

	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens -= 1
	return true
}

func (tl *TenantLimiter) cleanup() {
	ticker := time.NewTicker(cleanupEvery)
	defer ticker.Stop()
	for range ticker.C {
		now := tl.now().UTC()
		tl.limiters.Range(func(key, value any) bool {
			bucket, ok := value.(*tenantBucket)
			if !ok {
				tl.limiters.Delete(key)
				return true
			}
			bucket.mu.Lock()
			idle := now.Sub(bucket.lastSeen)
			bucket.mu.Unlock()
			if idle >= bucketTTL {
				tl.limiters.Delete(key)
			}
			return true
		})
	}
}

func orgKey(v any) string {
	switch t := v.(type) {
	case uuid.UUID:
		if t == uuid.Nil {
			return ""
		}
		return t.String()
	case string:
		return strings.TrimSpace(t)
	case interface{ String() string }:
		return strings.TrimSpace(t.String())
	default:
		return ""
	}
}
