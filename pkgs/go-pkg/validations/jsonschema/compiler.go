package jsonschema

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	js "github.com/santhosh-tekuri/jsonschema/v5"
)

type CompilerCache struct {
	mu    sync.Mutex
	cache map[string]*js.Schema
}

func NewCompilerCache() *CompilerCache {
	return &CompilerCache{cache: make(map[string]*js.Schema)}
}

func (c *CompilerCache) Compile(ctx context.Context, key string, schemaJSON []byte) (*js.Schema, error) {
	_ = ctx
	if len(schemaJSON) == 0 {
		return nil, errors.New("empty schema")
	}

	c.mu.Lock()
	if sch, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return sch, nil
	}
	c.mu.Unlock()

	compiler := js.NewCompiler()
	compiler.LoadURL = func(url string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("external refs not allowed: %s", url)
	}
	if err := compiler.AddResource("mem://schema.json", bytes.NewReader(schemaJSON)); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	sch, err := compiler.Compile("mem://schema.json")
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = sch
	c.mu.Unlock()

	return sch, nil
}

func Validate(schema *js.Schema, v any) error {
	return schema.Validate(v)
}
