package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"net/url"
	"strings"
	"time"

	"data-plane/internal/gateway/executor/circuitbreaker"
	"nexus/pkg/types"
)

type Executor struct {
	client       *nethttp.Client
	maxRespBytes int64
	retries      int
	cbRegistry   *circuitbreaker.Registry
}

type Options struct {
	Timeout          time.Duration
	MaxResponseBytes int64
	Retries          int
	// Transport overrides the default http.Transport. When nil, uses net/http default.
	Transport nethttp.RoundTripper
	// CheckRedirect overrides the default redirect policy. When nil, follows redirects.
	CheckRedirect func(req *nethttp.Request, via []*nethttp.Request) error
	CircuitBreaker circuitbreaker.Config
}

func NewExecutor(opts Options) *Executor {
	c := &nethttp.Client{Timeout: opts.Timeout}
	if opts.Transport != nil {
		c.Transport = opts.Transport
	}
	if opts.CheckRedirect != nil {
		c.CheckRedirect = opts.CheckRedirect
	}
	return &Executor{
		client:       c,
		maxRespBytes: opts.MaxResponseBytes,
		retries:      opts.Retries,
		cbRegistry:   circuitbreaker.NewRegistry(opts.CircuitBreaker),
	}
}

func (e *Executor) Execute(ctx context.Context, method, rawURL string, input map[string]any, headers map[string]string, maxRetries int) (any, int, *types.HTTPError) {
	cbKey := cbKeyFromURL(rawURL)
	cb := e.cbRegistry.Get(cbKey)

	if !cb.Allow() {
		return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeCircuitOpen, Message: "circuit breaker open for upstream"}
	}

	var lastErr *types.HTTPError
	backoff := 200 * time.Millisecond
	attempts := 1 + maxRetries
	for i := 0; i < attempts; i++ {
		res, status, he := e.executeOnce(ctx, method, rawURL, input, headers)
		if he == nil {
			cb.RecordSuccess()
			return res, status, nil
		}
		lastErr = he

		retryable := he.Code == types.ErrCodeNetworkError || he.Code == types.ErrCodeUpstream5xx
		if retryable {
			cb.RecordFailure()
		}
		if !retryable || i == attempts-1 {
			return nil, status, he
		}

		select {
		case <-ctx.Done():
			return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeTimeout, Message: "timeout"}
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, 0, lastErr
}

func cbKeyFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

func (e *Executor) executeOnce(ctx context.Context, method, rawURL string, input map[string]any, headers map[string]string) (any, int, *types.HTTPError) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeValidation, Message: "invalid url"}
	}

	var body io.Reader
	switch strings.ToUpper(method) {
	case "GET":
		q := u.Query()
		for k, v := range input {
			s, ok := primitiveToString(v)
			if !ok {
				return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeInvalidGETInput, Message: "GET input must be flat primitives"}
			}
			q.Set(k, s)
		}
		u.RawQuery = q.Encode()
	case "POST", "PUT", "PATCH", "DELETE":
		b, err := json.Marshal(input)
		if err != nil {
			return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeValidation, Message: "invalid input json"}
		}
		body = bytes.NewReader(b)
	default:
		return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeValidation, Message: "unsupported method"}
	}

	req, err := nethttp.NewRequestWithContext(ctx, strings.ToUpper(method), u.String(), body)
	if err != nil {
		return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeInternal, Message: "request build failed"}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeTimeout, Message: "timeout"}
		}
		return nil, 0, &types.HTTPError{Status: 0, Code: types.ErrCodeNetworkError, Message: "network error"}
	}
	defer resp.Body.Close()

	max := e.maxRespBytes
	if max <= 0 {
		max = 1048576
	}
	limited := io.LimitReader(resp.Body, max+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, resp.StatusCode, &types.HTTPError{Status: 0, Code: types.ErrCodeNetworkError, Message: "read error"}
	}
	if int64(len(b)) > max {
		return nil, resp.StatusCode, &types.HTTPError{Status: 0, Code: types.ErrCodeResponseTooLarge, Message: "response too large"}
	}

	if resp.StatusCode >= 500 {
		return nil, resp.StatusCode, &types.HTTPError{Status: 0, Code: types.ErrCodeUpstream5xx, Message: "upstream 5xx"}
	}
	if resp.StatusCode >= 400 {
		// Normalize 4xx as upstream errors but not retryable.
		return parseBody(b, resp.Header.Get("Content-Type")), resp.StatusCode, &types.HTTPError{Status: 0, Code: types.ErrCodeValidation, Message: "upstream 4xx"}
	}
	return parseBody(b, resp.Header.Get("Content-Type")), resp.StatusCode, nil
}

func parseBody(b []byte, contentType string) any {
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		var v any
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(&v); err == nil {
			return v
		}
	}
	s := string(b)
	if len(s) > 4096 {
		s = s[:4096]
	}
	return map[string]any{"raw": s}
}

func primitiveToString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), "."), true
	case json.Number:
		return t.String(), true
	default:
		return "", false
	}
}
