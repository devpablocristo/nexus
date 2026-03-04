package actionoverrides

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nexus-core/internal/gateway"
)

type Repository struct {
	baseURL     string
	internalKey string
	client      *http.Client
}

func NewRepository() *Repository {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXUS_SAAS_URL")), "/")
	key := strings.TrimSpace(os.Getenv("NEXUS_SAAS_INTERNAL_KEY"))
	timeoutMS := 300
	if raw := strings.TrimSpace(os.Getenv("NEXUS_SAAS_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutMS = parsed
		}
	}
	return &Repository{
		baseURL:     baseURL,
		internalKey: key,
		client:      &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
	}
}

type runtimeOverridesResponse struct {
	Deny              bool    `json:"deny"`
	DenyReason        string  `json:"deny_reason"`
	TenantRPMOverride *int    `json:"tenant_rpm_override"`
	ToolRPMOverride   *int    `json:"tool_rpm_override"`
}

// ResolveRuntimeOverrides calls nexus-saas over the internal contract.
// Failures are intentionally non-fatal so /v1/run can continue with defaults.
func (r *Repository) ResolveRuntimeOverrides(ctx context.Context, orgID uuid.UUID, toolName string) (gateway.RuntimeActionOverrides, error) {
	if r.baseURL == "" || r.internalKey == "" {
		return gateway.RuntimeActionOverrides{}, nil
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		r.baseURL+"/internal/runtime-overrides/"+orgID.String()+"/"+toolName,
		nil,
	)
	if err != nil {
		return gateway.RuntimeActionOverrides{}, nil
	}
	req.Header.Set("X-NEXUS-SAAS-KEY", r.internalKey)
	resp, err := r.client.Do(req)
	if err != nil {
		return gateway.RuntimeActionOverrides{}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return gateway.RuntimeActionOverrides{}, nil
	}
	var out runtimeOverridesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return gateway.RuntimeActionOverrides{}, nil
	}
	return gateway.RuntimeActionOverrides{
		Deny:              out.Deny,
		DenyReason:        out.DenyReason,
		TenantRPMOverride: out.TenantRPMOverride,
		ToolRPMOverride:   out.ToolRPMOverride,
	}, nil
}
