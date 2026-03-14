package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Executor struct {
	client *http.Client
}

func NewExecutor(timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Executor{
		client: &http.Client{Timeout: timeout},
	}
}

func (e *Executor) Execute(ctx context.Context, method, rawURL string, input map[string]any, headers map[string]string) (any, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid tool url: %w", err)
	}

	var body io.Reader
	switch strings.ToUpper(method) {
	case http.MethodGet:
		query := parsedURL.Query()
		for key, value := range input {
			query.Set(key, fmt.Sprint(value))
		}
		parsedURL.RawQuery = query.Encode()
	case http.MethodPost:
		payload, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("marshal input: %w", err)
		}
		body = bytes.NewReader(payload)
	default:
		return nil, fmt.Errorf("unsupported method %q", method)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), parsedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute upstream request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	return decodeResponse(resp)
}

func decodeResponse(resp *http.Response) (any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return nil, fmt.Errorf("decode upstream json: %w", err)
		}
		return value, nil
	}

	return map[string]any{"raw": string(body)}, nil
}
