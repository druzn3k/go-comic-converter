package comic

import "sync"

// registry is a concurrency-safe generic key-value store for extensibility points.
type registry[V any] struct {
	mu sync.RWMutex
	m  map[string]V
}

func newRegistry[V any]() *registry[V] {
	return &registry[V]{m: make(map[string]V)}
}

func (r *registry[V]) register(name string, v V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[name] = v
}

func (r *registry[V]) lookup(name string) (V, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.m[name]
	return v, ok
}

func (r *registry[V]) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.m))
	for k := range r.m {
		names = append(names, k)
	}
	return names
}
