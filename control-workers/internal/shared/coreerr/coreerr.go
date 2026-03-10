package coreerr

import "fmt"

// CoreError represents a non-2xx HTTP response from nexus-core.
type CoreError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *CoreError) Error() string {
	return fmt.Sprintf("core proxy %s %s failed status=%d body=%s", e.Method, e.Path, e.StatusCode, e.Body)
}

// IsRetryable returns true for server errors and rate-limiting.
func (e *CoreError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}
