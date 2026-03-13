package secrets

import (
	"context"

	secretdomain "nexus/v2/data-plane/internal/secrets/usecases/domain"
)

type InMemoryRepository struct {
	items map[string][]secretdomain.ToolSecret
}

func NewInMemoryRepository(items []secretdomain.ToolSecret) *InMemoryRepository {
	grouped := make(map[string][]secretdomain.ToolSecret)
	for _, item := range items {
		grouped[item.ToolID] = append(grouped[item.ToolID], item)
	}
	return &InMemoryRepository{items: grouped}
}

func (r *InMemoryRepository) ListForTool(_ context.Context, toolID string) ([]secretdomain.ToolSecret, error) {
	items := r.items[toolID]
	out := make([]secretdomain.ToolSecret, len(items))
	copy(out, items)
	return out, nil
}
