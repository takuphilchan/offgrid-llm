package inference

import (
	"context"
	"errors"
	"sync"
)

var ErrInferenceQueueFull = errors.New("inference queue is full; wait for active work to finish before retrying")

// LifecycleGate coordinates inference leases with model switches. Requests for
// the active model may run concurrently up to the configured limit; changing
// models waits for all active requests and is always exclusive.
type LifecycleGate struct {
	mu            sync.Mutex
	currentModel  string
	contextWindow int
	active        int
	maxConcurrent int
	switching     bool
	notify        chan struct{}
	waiters       []*leaseWaiter
	maxQueued     int
}

// Nonzero size guarantees distinct live waiter addresses in Go.
type leaseWaiter struct{ marker byte }

type LifecycleStatus struct {
	CurrentModel  string `json:"current_model,omitempty"`
	Active        int    `json:"active"`
	Capacity      int    `json:"capacity"`
	Switching     bool   `json:"switching"`
	ContextWindow int    `json:"context_window"`
	Queued        int    `json:"queued"`
	QueueCapacity int    `json:"queue_capacity"`
}

func NewLifecycleGate(maxConcurrent int) *LifecycleGate {
	return NewLifecycleGateWithQueue(maxConcurrent, 32)
}

func NewLifecycleGateWithQueue(maxConcurrent, maxQueued int) *LifecycleGate {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxQueued < 1 {
		maxQueued = 1
	}
	return &LifecycleGate{maxConcurrent: maxConcurrent, maxQueued: maxQueued, notify: make(chan struct{})}
}

// Acquire returns a release function after the requested model is ready. The
// switch callback runs without the gate lock and with no active inference.
func (g *LifecycleGate) Acquire(ctx context.Context, model string, switchModel func(context.Context, string) error) (func(), error) {
	return g.AcquireWithContext(ctx, model, 0, switchModel)
}

// Context changes require the same exclusive boundary as a model change.
func (g *LifecycleGate) AcquireWithContext(ctx context.Context, model string, contextWindow int, switchModel func(context.Context, string) error) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// FIFO prevents a steady stream of active-model requests from starving an
	// older model/context switch. A queue bound limits waiting goroutines/work.
	waiter := &leaseWaiter{}
	g.mu.Lock()
	if len(g.waiters) >= g.maxQueued {
		g.mu.Unlock()
		return nil, ErrInferenceQueueFull
	}
	g.waiters = append(g.waiters, waiter)
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.removeWaiterLocked(waiter)
		g.mu.Unlock()
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.mu.Lock()
		first := len(g.waiters) > 0 && g.waiters[0] == waiter
		if first && !g.switching && g.currentModel == model && g.contextWindow == contextWindow && g.active < g.maxConcurrent {
			g.active++
			g.removeWaiterLocked(waiter)
			g.mu.Unlock()
			return g.releaseFunc(), nil
		}
		if first && !g.switching && g.active == 0 {
			g.switching = true
			g.removeWaiterLocked(waiter)
			g.mu.Unlock()

			err := switchModel(ctx, model)

			g.mu.Lock()
			if err == nil {
				g.currentModel = model
				g.contextWindow = contextWindow
				if err = ctx.Err(); err == nil {
					g.active++
				}
			} else {
				g.currentModel = ""
				g.contextWindow = 0
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
	return LifecycleStatus{CurrentModel: g.currentModel, Active: g.active, Capacity: g.maxConcurrent, Switching: g.switching, ContextWindow: g.contextWindow, Queued: len(g.waiters), QueueCapacity: g.maxQueued}
}

func (g *LifecycleGate) removeWaiterLocked(waiter *leaseWaiter) {
	for i, item := range g.waiters {
		if item == waiter {
			copy(g.waiters[i:], g.waiters[i+1:])
			g.waiters[len(g.waiters)-1] = nil
			g.waiters = g.waiters[:len(g.waiters)-1]
			g.signalLocked()
			return
		}
	}
}

// Invalidate prevents a dead engine being reused. The next switch still waits
// for every active lease; it never tears a runtime down underneath a request.
func (g *LifecycleGate) Invalidate(model string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.currentModel == model {
		g.currentModel = ""
		g.contextWindow = 0
	}
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
