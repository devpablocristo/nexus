// Package coreproxy carries execution lease context over internal HTTP calls.
package coreproxy

import (
	"context"
	"net/http"
	"strings"
)

type executionLeaseHeadersKey struct{}

var forwardedExecutionLeaseHeaders = []string{
	"Authorization",
	"X-Nexus-Execution-Token",
	"X-Nexus-Lease-Id",
	"X-Nexus-Intent-Id",
	"X-Nexus-Credential-Mode",
	"X-Nexus-Tool-Name",
	"X-Nexus-Risk-Class",
	"X-Nexus-Credential-Scope",
	"X-Nexus-Credential-Provider",
	"X-Nexus-Target-Env",
}

// WithExecutionLeaseHeaders stores lease-bound HTTP headers in context so the
// coreproxy client can forward them to data-plane/operator bridges.
func WithExecutionLeaseHeaders(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	cloned := make(map[string]string, len(headers))
	for _, header := range forwardedExecutionLeaseHeaders {
		if value := strings.TrimSpace(headers[header]); value != "" {
			cloned[header] = value
		}
	}
	if len(cloned) == 0 {
		return ctx
	}
	return context.WithValue(ctx, executionLeaseHeadersKey{}, cloned)
}

func applyExecutionLeaseHeaders(ctx context.Context, req *http.Request) {
	headers, _ := ctx.Value(executionLeaseHeadersKey{}).(map[string]string)
	for _, header := range forwardedExecutionLeaseHeaders {
		if value := strings.TrimSpace(headers[header]); value != "" {
			req.Header.Set(header, value)
		}
	}
}
