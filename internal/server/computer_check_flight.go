package server

import (
	"context"
	"errors"
	"sync"
	"time"
)

type computerCheckKey struct {
	model, driver string
	contextWindow int
	budget        time.Duration
}

type computerCheckFlight struct {
	done     chan struct{}
	cancel   context.CancelFunc
	waiters  int
	finished bool
	result   computerModelCheck
}

// Synthetic checks contain no actor data and confer no authority. Only active
// work is shared, never a successful result or the caller's request context.
type computerCheckFlights struct {
	mu      sync.Mutex
	active  map[computerCheckKey]*computerCheckFlight
	workers sync.WaitGroup
	closing bool
}

func (g *computerCheckFlights) shutdown(ctx context.Context) error {
	g.mu.Lock()
	g.closing = true
	for _, f := range g.active {
		f.cancel()
	}
	g.mu.Unlock()
	done := make(chan struct{})
	go func() { g.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *computerCheckFlights) do(ctx, lifetime context.Context, key computerCheckKey, run func(context.Context) computerModelCheck) computerModelCheck {
	if err := ctx.Err(); err != nil {
		return cancelledComputerCheck(key.model, err)
	}
	if lifetime == nil {
		lifetime = context.Background()
	}
	if err := lifetime.Err(); err != nil {
		return cancelledComputerCheck(key.model, err)
	}
	g.mu.Lock()
	if g.closing {
		g.mu.Unlock()
		return cancelledComputerCheck(key.model, context.Canceled)
	}
	if g.active == nil {
		g.active = make(map[computerCheckKey]*computerCheckFlight)
	}
	f := g.active[key]
	if f == nil {
		// Admission has its own configured deadline; the probe deadline starts
		// only after admission. Last-observer departure and shutdown cancel both.
		checkCtx, cancel := context.WithCancel(lifetime)
		f = &computerCheckFlight{done: make(chan struct{}), cancel: cancel}
		g.active[key] = f
		g.workers.Add(1)
		go func(f *computerCheckFlight) {
			defer g.workers.Done()
			defer cancel()
			result := run(checkCtx)
			g.mu.Lock()
			f.result, f.finished = result, true
			// An abandoned old check must not remove its replacement.
			if g.active[key] == f {
				delete(g.active, key)
			}
			close(f.done)
			g.mu.Unlock()
		}(f)
	}
	f.waiters++
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		f.waiters--
		if f.waiters == 0 && !f.finished {
			if g.active[key] == f {
				delete(g.active, key)
			}
			f.cancel()
		}
		g.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return cancelledComputerCheck(key.model, ctx.Err())
	case <-f.done:
		// A simultaneous cancellation must not be projected as an approval to
		// continue a caller's saved task. The result itself is not a grant.
		if err := ctx.Err(); err != nil {
			return cancelledComputerCheck(key.model, err)
		}
		return cloneComputerCheck(f.result)
	}
}

func cancelledComputerCheck(model string, err error) computerModelCheck {
	result := computerModelCheck{Model: model, Code: "computer_model_check_cancelled", Message: "The model check was cancelled. This check did not execute a computer action.", Retryable: true}
	if errors.Is(err, context.DeadlineExceeded) {
		result.Code = "computer_model_check_wait_timeout"
		result.Message = "Timed out while waiting for the model check. Other work was not interrupted. This check did not execute a computer action."
	}
	return result
}

func cloneComputerCheck(result computerModelCheck) computerModelCheck {
	if result.Timings != nil {
		timings := *result.Timings
		result.Timings = &timings
	}
	if result.Runtime != nil {
		info := *result.Runtime
		if info.SupportsTools != nil {
			value := *info.SupportsTools
			info.SupportsTools = &value
		}
		if info.SupportsToolCalls != nil {
			value := *info.SupportsToolCalls
			info.SupportsToolCalls = &value
		}
		result.Runtime = &info
	}
	return result
}
