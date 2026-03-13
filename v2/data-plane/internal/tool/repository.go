package tool

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("tool not found")

type Kind string

const (
	KindHTTP Kind = "http"
)

type Definition struct {
	ID                 string
	Name               string
	Kind               Kind
	Method             string
	URL                string
	Enabled            bool
	RateLimitPerMinute int
	InputSchemaJSON    []byte
	OutputSchemaJSON   []byte
}

type InMemoryRepository struct {
	itemsByID   map[string]Definition
	itemsByName map[string]Definition
}

func NewInMemoryRepository(definitions []Definition) *InMemoryRepository {
	itemsByID := make(map[string]Definition, len(definitions))
	itemsByName := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		if definition.ID == "" {
			definition.ID = definition.Name
		}
		itemsByID[definition.ID] = definition
		itemsByName[definition.Name] = definition
	}
	return &InMemoryRepository{
		itemsByID:   itemsByID,
		itemsByName: itemsByName,
	}
}

func (r *InMemoryRepository) GetByID(_ context.Context, id string) (Definition, error) {
	definition, ok := r.itemsByID[id]
	if !ok {
		return Definition{}, ErrNotFound
	}
	return definition, nil
}

func (r *InMemoryRepository) GetByName(_ context.Context, name string) (Definition, error) {
	definition, ok := r.itemsByName[name]
	if !ok {
		return Definition{}, ErrNotFound
	}
	return definition, nil
}
