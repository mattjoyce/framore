package pipeline

import (
	"context"
	"sync"

	"github.com/mattjoyce/framore/internal/batch"
)

type Stage interface {
	Name() string
	Enabled(b *batch.Batch) bool
	// SupportsNoWait reports whether this stage can run under --no-wait.
	// Return false for stages that block on synchronous external work
	// (e.g. direct HTTP per file) or depend on other stages' results.
	SupportsNoWait() bool
	Run(ctx context.Context, b *batch.Batch, results *Results) error
}

type Results struct {
	mu   sync.RWMutex
	data map[string]map[string]any
}

func NewResults() *Results {
	return &Results{data: make(map[string]map[string]any)}
}

func (r *Results) Set(stage, key string, val any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data[stage] == nil {
		r.data[stage] = make(map[string]any)
	}
	r.data[stage][key] = val
}

func (r *Results) Get(stage, key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.data[stage]; ok {
		v, ok := m[key]
		return v, ok
	}
	return nil, false
}

func (r *Results) AllForStage(stage string) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.data[stage]; ok {
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		return cp
	}
	return nil
}
