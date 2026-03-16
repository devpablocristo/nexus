package httpserver

import (
	"net/http"
	"os"
	"strings"
)

type SecurityConfig struct {
	AllowedOrigins   []string
	HSTSMaxAge       string
	AllowCredentials bool
}

func SecurityConfigFromEnv() SecurityConfig {
	return SecurityConfig{
		AllowedOrigins:   splitList(os.Getenv("NEXUS_CORS_ALLOWED_ORIGINS")),
		HSTSMaxAge:       strings.TrimSpace(os.Getenv("NEXUS_HSTS_MAX_AGE")),
		AllowCredentials: parseBool(os.Getenv("NEXUS_CORS_ALLOW_CREDENTIALS")),
	}
}

func SecurityMiddleware(cfg SecurityConfig, next http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	allowAnyOrigin := false
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			allowAnyOrigin = true
			continue
		}
		allowedOrigins[origin] = struct{}{}
	}

	allowedHeaders := strings.Join([]string{
		"Authorization",
		"Content-Type",
		"X-API-Key",
		"X-Request-Id",
	}, ", ")
	allowedMethods := strings.Join([]string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.TrimSpace(cfg.HSTSMaxAge) != "" && isHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age="+cfg.HSTSMaxAge+"; includeSubDomains")
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && (allowAnyOrigin || originAllowed(origin, allowedOrigins)) {
			w.Header().Add("Vary", "Origin")
			if allowAnyOrigin && !cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if r.Method == http.MethodOptions && origin != "" {
			if allowAnyOrigin || originAllowed(origin, allowedOrigins) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	return items
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	_, ok := allowed[origin]
	return ok
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}
