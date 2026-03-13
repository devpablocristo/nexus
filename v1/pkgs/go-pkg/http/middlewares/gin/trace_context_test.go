package ginmw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"nexus/pkg/types"
)

func TestTraceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TraceContext())
	r.GET("/trace", func(c *gin.Context) {
		traceID, _ := c.Get(string(types.CtxKeyTraceID))
		spanID, _ := c.Get(string(types.CtxKeySpanID))
		c.JSON(http.StatusOK, gin.H{
			"trace_id": traceID,
			"span_id":  spanID,
		})
	})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		SpanID:     trace.SpanID{2, 2, 2, 2, 2, 2, 2, 2},
		TraceFlags: trace.FlagsSampled,
		Remote:     false,
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req = req.WithContext(trace.ContextWithSpanContext(context.Background(), sc))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body["trace_id"]; got != sc.TraceID().String() {
		t.Fatalf("unexpected trace_id: got=%q want=%q", got, sc.TraceID().String())
	}
	if got := body["span_id"]; got != sc.SpanID().String() {
		t.Fatalf("unexpected span_id: got=%q want=%q", got, sc.SpanID().String())
	}
}
