package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/server"
)

type cleanupStep struct {
	name    string
	close   func() error
	barrier bool
}

// cleanupStack records ownership immediately after each successful acquisition.
// No resource is published to Runtime until all required construction succeeds.
type cleanupStack struct {
	steps []cleanupStep
}

func (s *cleanupStack) add(name string, close func() error) {
	if close == nil {
		return
	}
	s.steps = append(s.steps, cleanupStep{name: name, close: close})
}

// addBarrier marks a dependency whose failed close makes releasing earlier
// owners unsafe. Provider admission must drain before its model cache and
// workspace can be released.
func (s *cleanupStack) addBarrier(name string, close func() error) {
	if close == nil {
		return
	}
	s.add(name, close)
	s.steps[len(s.steps)-1].barrier = true
}

// promote moves an already-owned resource to the end of acquisition order so
// it is stopped first. This is used after late model-cache construction: task
// cancellation and provider admission must stop before llama's cache unloads.
func (s *cleanupStack) promote(name string) error {
	index := -1
	for i, step := range s.steps {
		if step.name == name {
			if index >= 0 {
				return fmt.Errorf("cleanup owner %q is ambiguous", name)
			}
			index = i
		}
	}
	if index < 0 {
		return fmt.Errorf("cleanup owner %q is missing", name)
	}
	step := s.steps[index]
	copy(s.steps[index:], s.steps[index+1:])
	s.steps[len(s.steps)-1] = step
	return nil
}

func (s *cleanupStack) close() error {
	var result error
	for i := len(s.steps) - 1; i >= 0; i-- {
		step := s.steps[i]
		if err := step.close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close %s: %w", step.name, err))
			if step.barrier {
				// The remaining owners are deliberately retained. Process exit
				// is safer than unloading a model under an unconfirmed lease.
				s.steps = s.steps[:i+1]
				return result
			}
		}
	}
	s.steps = nil
	return result
}

func (s *cleanupStack) rollback(cause error) error {
	return errors.Join(cause, s.close())
}

// Runtime is the process owner for one composed service. The transport is
// drained before the resources it borrows are released.
type Runtime struct {
	transport *server.Server
	owned     cleanupStack
	closeOnce sync.Once
	closeErr  error
}

// Run serves until shutdown or caller cancellation, then releases the graph.
// A normal caller cancellation is not reported as a server failure.
func (r *Runtime) Run(ctx context.Context) error {
	if r == nil || r.transport == nil {
		return fmt.Errorf("application transport is unavailable")
	}
	err := r.transport.Serve(ctx)
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		err = nil
	}
	return errors.Join(err, r.Close())
}

// Close is idempotent. If the HTTP drain is uncertain, owned state remains
// retained rather than being released while active requests may still write.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.transport != nil {
			if err := r.transport.Close(); err != nil {
				r.closeErr = fmt.Errorf("drain HTTP transport: %w", err)
				return
			}
		}
		r.closeErr = r.owned.close()
	})
	return r.closeErr
}
