package llm

import (
	"context"

	"nexus-core/pkg/types"
)

type cloudProvider struct{}

func NewCloudProvider(cfg Config) Provider {
	_ = cfg
	return &cloudProvider{}
}

func (c *cloudProvider) Generate(_ context.Context, _ ProviderRequest) (map[string]any, error) {
	return nil, types.NewHTTPError(501, types.ErrCodeInternal, "cloud llm provider not configured")
}
