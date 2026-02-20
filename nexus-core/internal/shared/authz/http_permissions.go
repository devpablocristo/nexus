package authz

import (
	"os"
	"strconv"
	"sync"
)

var (
	legacyScopeFallbackOnce sync.Once
	legacyScopeFallback     bool
)

func legacyFallbackEnabled() bool {
	legacyScopeFallbackOnce.Do(func() {
		v := os.Getenv("NEXUS_AUTH_LEGACY_SCOPE_FALLBACK")
		if v == "" {
			legacyScopeFallback = true
			return
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			legacyScopeFallback = true
			return
		}
		legacyScopeFallback = b
	})
	return legacyScopeFallback
}

// CanAccess keeps backward compatibility for legacy API keys without scopes:
// - admin/secops role: always allowed
// - no scopes present: allowed (legacy mode)
// - scopes present: required scope must be present
func CanAccess(scopes []string, role *string, required string) bool {
	if IsRole(role, "admin", "secops") {
		return true
	}
	if len(scopes) == 0 && legacyFallbackEnabled() {
		return true
	}
	return HasScope(scopes, required)
}
