package jsonschema

import (
	"context"
	"testing"
)

func TestCompileAndValidate(t *testing.T) {
	cache := NewCompilerCache()
	s, err := cache.Compile(context.Background(), "k1", []byte(`{"type":"object","properties":{"a":{"type":"number"}},"required":["a"]}`))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := Validate(s, map[string]any{"a": 1.0}); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if err := Validate(s, map[string]any{"b": 1.0}); err == nil {
		t.Fatalf("expected invalid")
	}
}
