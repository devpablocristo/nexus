package circuitbreaker

import "sync"

// Registry manages per-key circuit breakers (typically keyed by tool URL host).
type Registry struct {
	mu       sync.Mutex
	breakers map[string]*Breaker
	cfg      Config
}

func NewRegistry(cfg Config) *Registry {
	return &Registry{
		breakers: make(map[string]*Breaker),
		cfg:      cfg,
	}
}

func (r *Registry) Get(key string) *Breaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[key]
	if !ok {
		b = New(r.cfg)
		r.breakers[key] = b
	}
	return b
}

func (r *Registry) State(key string) State {
	r.mu.Lock()
	b, ok := r.breakers[key]
	r.mu.Unlock()
	if !ok {
		return StateClosed
	}
	return b.State()
}
