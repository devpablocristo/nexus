package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const RequestIDHeader = "X-Request-Id"

type contextKey string

const (
	requestIDContextKey contextKey = "nexus.observability.request_id"
	loggerContextKey    contextKey = "nexus.observability.logger"
)

// NewJSONLogger builds a structured logger for one service.
func NewJSONLogger(service string) *slog.Logger {
	return NewJSONLoggerWriter(service, os.Stdout)
}

// NewJSONLoggerWriter builds a structured logger writing to the provided destination.
func NewJSONLoggerWriter(service string, w io.Writer) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{})).With("service", service)
}

// Middleware assigns request IDs, injects a request-scoped logger, and emits one access log per request.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return MiddlewareWithMetrics(logger, nil, next)
}

// MiddlewareWithMetrics extends the request logging middleware with Prometheus RED metrics.
func MiddlewareWithMetrics(logger *slog.Logger, metrics *Metrics, next http.Handler) http.Handler {
	if logger == nil {
		logger = NewJSONLogger("unknown")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}

		start := time.Now()
		w.Header().Set(RequestIDHeader, requestID)

		ctx := ContextWithRequestID(r.Context(), requestID)
		requestLogger := logger.With("request_id", requestID)
		ctx = ContextWithLogger(ctx, requestLogger)
		*r = *r.WithContext(ctx)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := routeLabel(r)
		if metrics != nil {
			metrics.ObserveHTTPRequest(r, rec.status, time.Since(start))
		}

		attrs := []any{
			"event", "http_request_completed",
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		}
		requestLogger.Info("http request completed", attrs...)
	})
}

// ContextWithRequestID stores the request ID in context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// RequestIDFromContext returns the request ID, if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDContextKey).(string)
	if !ok || requestID == "" {
		return "", false
	}
	return requestID, true
}

// ContextWithLogger stores a request-scoped logger in context.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey, logger)
}

// LoggerFromContext returns the request-scoped logger when available.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerContextKey).(*slog.Logger)
	if !ok || logger == nil {
		return slog.Default()
	}
	return logger
}

// ApplyRequestID propagates the inbound request ID to outbound requests.
func ApplyRequestID(r *http.Request, ctx context.Context) {
	if r == nil {
		return
	}
	if requestID, ok := RequestIDFromContext(ctx); ok {
		r.Header.Set(RequestIDHeader, requestID)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf[:])
}
