// Package coreproxy provides contract-driven adapters to Nexus Core.
package coreproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"nexus-control-operators/internal/shared/coreerr"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	log     zerolog.Logger
}

func NewClient(baseURL, apiKey string, timeout time.Duration, log zerolog.Logger) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: timeout},
		log:     log.With().Str("component", "coreproxy").Logger(),
	}
}

func (c *Client) DoJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	if c.baseURL == "" || c.apiKey == "" {
		return fmt.Errorf("core proxy client is not configured")
	}
	var body io.Reader
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-NEXUS-AI-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
