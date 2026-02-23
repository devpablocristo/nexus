package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nexus-core/pkg/types"
	"nexus-core/pkg/validations/jsonschema"
)

type Request struct {
	Task  string
	Input map[string]any
}

type ProviderRequest struct {
	Task  string
	Model string
	Input map[string]any
}

type Provider interface {
	Generate(ctx context.Context, req ProviderRequest) (map[string]any, error)
}

type Client interface {
	Generate(ctx context.Context, req Request) (map[string]any, error)
	GenerateStrict(ctx context.Context, req Request, schemaFile string) (map[string]any, error)
}

type client struct {
	cfg       Config
	provider  Provider
	cache     *jsonschema.CompilerCache
}

func NewClient(cfg Config, cache *jsonschema.CompilerCache) Client {
	if cache == nil {
		cache = jsonschema.NewCompilerCache()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if strings.TrimSpace(cfg.SchemaDir) == "" {
		cfg.SchemaDir = filepath.Join("internal", "ops", "schemas", "llm")
	}
	return &client{
		cfg:      cfg,
		provider: providerFromConfig(cfg),
		cache:    cache,
	}
}

func (c *client) GenerateStrict(ctx context.Context, req Request, schemaFile string) (map[string]any, error) {
	if strings.TrimSpace(req.Task) == "" {
		return nil, types.NewHTTPError(400, types.ErrCodeValidation, "task is required")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	raw, err := c.provider.Generate(ctx, ProviderRequest{
		Task:  req.Task,
		Model: c.cfg.Model,
		Input: req.Input,
	})
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, types.NewHTTPError(502, types.ErrCodeValidation, "llm provider returned empty payload")
	}

	schemaPath := filepath.Join(c.cfg.SchemaDir, schemaFile)
	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read llm schema %s: %w", schemaPath, err)
	}
	sch, err := c.cache.Compile(ctx, "llm:"+schemaFile, schemaRaw)
	if err != nil {
		return nil, types.NewHTTPError(500, types.ErrCodeSchemaInvalid, "llm schema compile failed")
	}
	if err := jsonschema.Validate(sch, raw); err != nil {
		return nil, types.NewHTTPError(400, types.ErrCodeValidation, "llm output schema validation failed: "+err.Error())
	}
	return raw, nil
}

func (c *client) Generate(ctx context.Context, req Request) (map[string]any, error) {
	if strings.TrimSpace(req.Task) == "" {
		return nil, types.NewHTTPError(400, types.ErrCodeValidation, "task is required")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	return c.provider.Generate(ctx, ProviderRequest{
		Task:  req.Task,
		Model: c.cfg.Model,
		Input: req.Input,
	})
}

func providerFromConfig(cfg Config) Provider {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "ollama":
		return NewOllamaProvider(cfg)
	case "cloud":
		return NewCloudProvider(cfg)
	case "mock":
		fallthrough
	default:
		return NewMockProvider()
	}
}

func pretty(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
