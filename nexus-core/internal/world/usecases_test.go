package world

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nexus-core/pkg/types"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestServiceListRuns_PropagatesHeadersAndOrgID(t *testing.T) {
	orgID := uuid.New()
	var gotReqID string
	var gotKey string
	var gotOrgID string

	svc := &Usecases{
		cfg: Config{
			BaseURL:     "http://sim-engine:8087",
			InternalKey: "internal-secret",
			Timeout:     2 * time.Second,
		},
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotReqID = r.Header.Get(requestIDHeader)
				gotKey = r.Header.Get(internalKeyHeader)
				gotOrgID = r.URL.Query().Get("org_id")
				body := `{"items":[],"next_cursor":""}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}

	if _, err := svc.ListRuns(context.Background(), orgID, "req-world-1", 100, ""); err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if gotReqID != "req-world-1" {
		t.Fatalf("expected %s header, got %q", requestIDHeader, gotReqID)
	}
	if gotKey != "internal-secret" {
		t.Fatalf("expected %s header to be propagated", internalKeyHeader)
	}
	if gotOrgID != orgID.String() {
		t.Fatalf("expected org_id query to be propagated, got %q", gotOrgID)
	}
}

func TestServiceListRuns_MapsUnauthorized(t *testing.T) {
	orgID := uuid.New()
	svc := &Usecases{
		cfg: Config{
			BaseURL: "http://sim-engine:8087",
			Timeout: 2 * time.Second,
		},
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				raw, _ := json.Marshal(map[string]any{
					"error": map[string]any{"message": "forbidden"},
				})
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(raw))),
				}, nil
			}),
		},
	}
	_, err := svc.ListRuns(context.Background(), orgID, "req-world-2", 10, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	var he types.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.Code != types.ErrCodeUnauthorized {
		t.Fatalf("expected %s code, got %s", types.ErrCodeUnauthorized, he.Code)
	}
	if he.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", he.Status)
	}
}
