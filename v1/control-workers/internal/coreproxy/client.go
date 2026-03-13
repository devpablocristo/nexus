// Package coreproxy provides HTTP clients for protected nexus-core routes.
package coreproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"control-workers/internal/shared/coreerr"
	"control-workers/internal/shared/metrics"
)

type Client struct {
	baseURL          string
	apiKey           string
	http             *http.Client
	log              zerolog.Logger
	retryAttempts    int
	retryBackoffBase time.Duration
}

func NewClient(baseURL, apiKey string, timeout time.Duration, log zerolog.Logger) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Client{
		baseURL:          strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:           strings.TrimSpace(apiKey),
		http:             &http.Client{Timeout: timeout},
		log:              log.With().Str("component", "coreproxy").Logger(),
		retryAttempts:    3,
		retryBackoffBase: time.Second,
	}
}

func (c *Client) DoJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	if c.baseURL == "" || c.apiKey == "" {
		return fmt.Errorf("core proxy client is not configured")
	}

	var rawBody []byte
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		rawBody = raw
	}

	for attempt := 0; ; attempt++ {
		err := c.doJSONOnce(ctx, method, path, rawBody, out)
		if err == nil {
			return nil
		}

		if !c.shouldRetry(err) || attempt >= c.retryAttempts {
			return err
		}

		wait := c.retryBackoffBase << uint(attempt)
		c.log.Warn().
			Err(err).
			Str("method", method).
			Str("path", path).
			Int("attempt", attempt+1).
			Dur("backoff", wait).
			Msg("core request failed, retrying")
		if err := sleepWithContext(ctx, wait); err != nil {
			return err
		}
	}
}

func (c *Client) doJSONOnce(ctx context.Context, method, path string, rawBody []byte, out any) error {
	var body io.Reader
	if len(rawBody) > 0 {
		body = bytes.NewReader(rawBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-NEXUS-AI-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	applyExecutionLeaseHeaders(ctx, req)

	resp, err := c.http.Do(req)
	if err != nil {
		metrics.CoreRequests.WithLabelValues(method, "error").Inc()
		return err
	}
	defer resp.Body.Close()

	metrics.CoreRequests.WithLabelValues(method, strconv.Itoa(resp.StatusCode)).Inc()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return &coreerr.CoreError{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Body:       string(payload),
		}
	}
	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var coreErr *coreerr.CoreError
	if errors.As(err, &coreErr) {
		return coreErr.IsRetryable()
	}
	return true
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("nexus-core readyz returned %d", resp.StatusCode)
	}
	return nil
}
