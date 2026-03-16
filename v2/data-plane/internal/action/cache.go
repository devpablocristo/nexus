package action

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

// CacheConfig controls TTLs for the degradation cache.
type CacheConfig struct {
	ResourceSoftTTL  time.Duration // refresh if older than this (default 30s)
	ResourceHardTTL  time.Duration // fail closed if older than this (default 15m)
	PolicySoftTTL    time.Duration // refresh if older than this (default 30s)
	PolicyHardTTL    time.Duration // fail closed if older than this (default 5m)
}

// DefaultCacheConfig returns sane defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		ResourceSoftTTL: 30 * time.Second,
		ResourceHardTTL: 15 * time.Minute,
		PolicySoftTTL:   30 * time.Second,
		PolicyHardTTL:   5 * time.Minute,
	}
}

type cacheEntry[T any] struct {
	value     T
	fetchedAt time.Time
	expiresAt time.Time
	version   int64
}

func (e cacheEntry[T]) isStale(softTTL time.Duration) bool {
	return time.Since(e.fetchedAt) > softTTL
}

func (e cacheEntry[T]) isExpired(hardTTL time.Duration) bool {
	return time.Since(e.fetchedAt) > hardTTL
}

// degradationKey is the context key for per-request degradation tracking.
type degradationKey struct{}

// DegradationCollector tracks degradation flags for a single request.
// It is stored in context.Context and is request-local (no shared state).
type DegradationCollector struct {
	resourceDegraded bool
	policiesDegraded bool
}

// IsDegraded returns true if either resource or policy resolution used stale cache.
func (d *DegradationCollector) IsDegraded() bool {
	return d.resourceDegraded || d.policiesDegraded
}

// WithDegradationCollector returns a new context with a fresh DegradationCollector.
func WithDegradationCollector(ctx context.Context) context.Context {
	return context.WithValue(ctx, degradationKey{}, &DegradationCollector{})
}

// DegradationFromContext extracts the DegradationCollector from context, or nil.
func DegradationFromContext(ctx context.Context) *DegradationCollector {
	d, _ := ctx.Value(degradationKey{}).(*DegradationCollector)
	return d
}

// CachingResourceResolver wraps a ResourceResolver with an in-memory cache
// and graceful degradation when the upstream is unavailable.
type CachingResourceResolver struct {
	upstream ResourceResolver
	config   CacheConfig
	logger   *slog.Logger

	mu      sync.RWMutex
	entries map[string]cacheEntry[actiondomain.ProtectedResource]
	version int64
}

// NewCachingResourceResolver wraps an upstream resolver with caching.
func NewCachingResourceResolver(upstream ResourceResolver, config CacheConfig, logger *slog.Logger) *CachingResourceResolver {
	return &CachingResourceResolver{
		upstream: upstream,
		config:   config,
		logger:   logger,
		entries:  make(map[string]cacheEntry[actiondomain.ProtectedResource]),
	}
}

func (c *CachingResourceResolver) GetByID(ctx context.Context, resourceID string) (actiondomain.ProtectedResource, error) {
	// Try cache first.
	c.mu.RLock()
	entry, cached := c.entries[resourceID]
	c.mu.RUnlock()

	// If cache is fresh (within soft TTL), return it directly.
	if cached && !entry.isStale(c.config.ResourceSoftTTL) {
		return entry.value, nil
	}

	// Try upstream.
	resource, err := c.upstream.GetByID(ctx, resourceID)
	if err == nil {
		now := time.Now().UTC()
		c.mu.Lock()
		c.version++
		c.entries[resourceID] = cacheEntry[actiondomain.ProtectedResource]{
			value:     resource,
			fetchedAt: now,
			expiresAt: now.Add(c.config.ResourceHardTTL),
			version:   c.version,
		}
		c.mu.Unlock()
		return resource, nil
	}

	// Upstream failed. Use cache if within hard TTL.
	if cached && !entry.isExpired(c.config.ResourceHardTTL) {
		c.logger.WarnContext(ctx, "control-plane unavailable, using cached resource",
			"resource_id", resourceID,
			"cache_age", time.Since(entry.fetchedAt).String(),
			"expires_at", entry.expiresAt.Format(time.RFC3339),
			"version", entry.version,
			"error", err.Error(),
		)
		if d := DegradationFromContext(ctx); d != nil {
			d.resourceDegraded = true
		}
		return entry.value, nil
	}

	// Cache miss or expired beyond hard TTL: fail closed.
	return actiondomain.ProtectedResource{}, fmt.Errorf("control-plane unavailable and no valid cache for resource %s: %w", resourceID, err)
}

// CachingPolicySource wraps a PolicySource with an in-memory cache
// and graceful degradation when the upstream is unavailable.
type CachingPolicySource struct {
	upstream PolicySource
	config   CacheConfig
	logger   *slog.Logger

	mu      sync.RWMutex
	entries map[string]cacheEntry[[]ActionPolicy]
	version int64
}

// NewCachingPolicySource wraps an upstream policy source with caching.
func NewCachingPolicySource(upstream PolicySource, config CacheConfig, logger *slog.Logger) *CachingPolicySource {
	return &CachingPolicySource{
		upstream: upstream,
		config:   config,
		logger:   logger,
		entries:  make(map[string]cacheEntry[[]ActionPolicy]),
	}
}

func policyKey(actionType, resourceType string) string {
	return actionType + "|" + resourceType
}

func (c *CachingPolicySource) List(ctx context.Context, actionType, resourceType string) ([]ActionPolicy, error) {
	key := policyKey(actionType, resourceType)

	// Try cache first.
	c.mu.RLock()
	entry, cached := c.entries[key]
	c.mu.RUnlock()

	// If cache is fresh (within soft TTL), return it directly.
	if cached && !entry.isStale(c.config.PolicySoftTTL) {
		return entry.value, nil
	}

	// Try upstream.
	policies, err := c.upstream.List(ctx, actionType, resourceType)
	if err == nil {
		now := time.Now().UTC()
		c.mu.Lock()
		c.version++
		c.entries[key] = cacheEntry[[]ActionPolicy]{
			value:     policies,
			fetchedAt: now,
			expiresAt: now.Add(c.config.PolicyHardTTL),
			version:   c.version,
		}
		c.mu.Unlock()
		return policies, nil
	}

	// Upstream failed. Use cache if within hard TTL.
	if cached && !entry.isExpired(c.config.PolicyHardTTL) {
		c.logger.WarnContext(ctx, "control-plane unavailable, using cached policies",
			"action_type", actionType,
			"resource_type", resourceType,
			"cache_age", time.Since(entry.fetchedAt).String(),
			"expires_at", entry.expiresAt.Format(time.RFC3339),
			"version", entry.version,
			"error", err.Error(),
		)
		if d := DegradationFromContext(ctx); d != nil {
			d.policiesDegraded = true
		}
		return entry.value, nil
	}

	// Cache miss or expired beyond hard TTL: fail closed.
	return nil, fmt.Errorf("control-plane unavailable and no valid cache for policies %s/%s: %w", actionType, resourceType, err)
}
