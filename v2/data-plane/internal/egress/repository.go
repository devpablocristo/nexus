package egress

import (
	"context"
	"strings"
	"sync"
)

type Rule struct {
	ToolID  string
	Host    string
	Enabled bool
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[string]map[string]bool
}

func NewInMemoryRepository(rules []Rule) *InMemoryRepository {
	items := map[string]map[string]bool{}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		toolID := strings.TrimSpace(rule.ToolID)
		host := normalizeHost(rule.Host)
		if toolID == "" || host == "" {
			continue
		}
		if items[toolID] == nil {
			items[toolID] = map[string]bool{}
		}
		items[toolID][host] = true
	}
	return &InMemoryRepository{items: items}
}

func (r *InMemoryRepository) HasAny(_ context.Context, toolID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.items[toolID]) > 0, nil
}

func (r *InMemoryRepository) ExistsHost(_ context.Context, toolID, host string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.items[toolID][normalizeHost(host)], nil
}
