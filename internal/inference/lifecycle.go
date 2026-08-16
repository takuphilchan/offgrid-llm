package inference

import (
	"context"
	"sync"
)

// LifecycleGate coordinates inference leases with model switches. Requests for
// the active model may run concurrently up to the configured limit; changing
// models waits for all active requests and is always exclusive.
type LifecycleGate struct {
	mu            sync.Mutex
	currentModel  string
	active        int
	maxConcurrent int
	switching     bool
	notify        chan struct{}
}

type LifecycleStatus struct {
	CurrentModel string `json:"current_model,omitempty"`
	Active       int    `json:"active"`
	Capacity     int    `json:"capacity"`
	Switching    bool   `json:"switching"`
}

func NewLifecycleGate(maxConcurrent int) *LifecycleGate {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &LifecycleGate{maxConcurrent: maxConcurrent, notify: make(chan struct{})}
}

// Acquire returns a release function after the requested model is ready. The
// switch callback runs without the gate lock and with no active inference.
func (g *LifecycleGate) Acquire(ctx context.Context, model string, switchModel func(context.Context, string) error) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		if !g.switching && g.currentModel == model && g.active < g.maxConcurrent {
			g.active++
			g.mu.Unlock()
			return g.releaseFunc(), nil
		}
		if !g.switching && g.active == 0 {
			g.switching = true
			g.mu.Unlock()

			err := switchModel(ctx, model)

			g.mu.Lock()
			if err == nil {
				g.currentModel = model
				g.active++
			}
			g.switching = false
			g.signalLocked()
			g.mu.Unlock()
			if err != nil {
				return nil, err
			}
			return g.releaseFunc(), nil
		}
		notify := g.notify
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-notify:
		}
	}
}

func (g *LifecycleGate) Status() LifecycleStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	return LifecycleStatus{CurrentModel: g.currentModel, Active: g.active, Capacity: g.maxConcurrent, Switching: g.switching}
}

func (g *LifecycleGate) releaseFunc() func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			if g.active > 0 {
				g.active--
			}
			g.signalLocked()
			g.mu.Unlock()
		})
	}
}

func (g *LifecycleGate) signalLocked() {
	close(g.notify)
	g.notify = make(chan struct{})
}
