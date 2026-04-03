package callbacks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	EventApprovalPending  = "approval_pending"
	EventApprovalResolved = "approval_resolved"
)

type ApprovalEvent struct {
	Event          string  `json:"event"`
	ApprovalID     string  `json:"approval_id,omitempty"`
	OrgID          string  `json:"org_id,omitempty"`
	RequestID      string  `json:"request_id"`
	Decision       string  `json:"decision,omitempty"`
	DecidedBy      string  `json:"decided_by,omitempty"`
	DecisionNote   string  `json:"decision_note,omitempty"`
	ActionType     string  `json:"action_type,omitempty"`
	TargetResource string  `json:"target_resource,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	RiskLevel      string  `json:"risk_level,omitempty"`
	AISummary      *string `json:"ai_summary,omitempty"`
	CreatedAt      string  `json:"created_at,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	DecidedAt      *string `json:"decided_at,omitempty"`
}

type ApprovalPublisher interface {
	Publish(ctx context.Context, event ApprovalEvent) error
}

type HTTPApprovalPublisher struct {
	client       *http.Client
	token        string
	pendingURLs  []string
	resolvedURLs []string
}

func NewHTTPApprovalPublisher(token string, pendingURLs, resolvedURLs []string) *HTTPApprovalPublisher {
	return &HTTPApprovalPublisher{
		client:       &http.Client{Timeout: 5 * time.Second},
		token:        strings.TrimSpace(token),
		pendingURLs:  normalizeURLs(pendingURLs),
		resolvedURLs: normalizeURLs(resolvedURLs),
	}
}

func (p *HTTPApprovalPublisher) Publish(ctx context.Context, event ApprovalEvent) error {
	targets := p.urlsFor(event.Event)
	if len(targets) == 0 {
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal callback event: %w", err)
	}
	var firstErr error
	for _, target := range targets {
		if err := p.post(ctx, target, payload); err != nil {
			slog.Error("approval callback failed", "url", target, "event", event.Event, "request_id", event.RequestID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (p *HTTPApprovalPublisher) post(ctx context.Context, target string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.token != "" {
		req.Header.Set("X-Internal-Service-Token", p.token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("status %d body %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (p *HTTPApprovalPublisher) urlsFor(event string) []string {
	switch strings.TrimSpace(event) {
	case EventApprovalPending:
		return p.pendingURLs
	case EventApprovalResolved:
		return p.resolvedURLs
	default:
		return nil
	}
}

func normalizeURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		url := strings.TrimSpace(raw)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}
	return out
}
