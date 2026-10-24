package inference

import (
	"context"
	"errors"
	"time"
)

type legacyPreparationKey struct {
	gate    *LifecycleGate
	model   string
	context int
}

type legacyPreparation struct {
	done     chan struct{}
	cancel   context.CancelFunc
	waiters  int
	retiring bool
	err      error
	timing   ProviderTiming
}

// Register demand before the numeric generation queue. Otherwise a cancelled
// auxiliary request cannot see the main request waiting behind it and tears down
// the same model repeatedly. Preparation owns its scheduler lease, not the first
// HTTP request. No prompt, credential or generation is shared between callers.
func (s *ProviderScheduler) prepareLegacyModel(ctx context.Context, gate *LifecycleGate, model string, window int, alive func() bool, load func(context.Context, string) error) (ProviderTiming, error) {
	started := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return ProviderTiming{}, err
		}
		status := gate.Status()
		if status.CurrentModel == model && contextCompatible(status.ContextWindow, window) && (alive == nil || alive()) {
			return ProviderTiming{}, nil
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return ProviderTiming{}, errors.New("provider scheduler is closed")
		}
		var pending *legacyPreparation
		total := 0
		for key, p := range s.legacyPreparing {
			total += p.waiters
			if key.gate == gate && key.model == model && contextCompatible(key.context, window) {
				pending = p
			}
		}
		if pending != nil && pending.retiring {
			done := pending.done
			s.mu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return ProviderTiming{}, ctx.Err()
			case <-s.closeCtx.Done():
				return ProviderTiming{}, errors.New("provider scheduler is closed")
			}
		}
		if total >= 32 {
			s.mu.Unlock()
			return ProviderTiming{}, ErrInferenceQueueFull
		}
		if pending == nil {
			if s.legacyPreparing == nil {
				s.legacyPreparing = make(map[legacyPreparationKey]*legacyPreparation)
			}
			// A service-specific model-load deadline may be shorter. This outer
			// bound prevents abandoned preparation even for standalone adapters.
			prepareCtx, cancel := context.WithTimeout(s.closeCtx, 5*time.Minute)
			pending = &legacyPreparation{done: make(chan struct{}), cancel: cancel}
			key := legacyPreparationKey{gate: gate, model: model, context: window}
			s.legacyPreparing[key] = pending
			go s.runLegacyPreparation(prepareCtx, key, pending, alive, load)
		}
		pending.waiters++
		s.mu.Unlock()
		select {
		case <-pending.done:
			if err := ctx.Err(); err != nil {
				return ProviderTiming{}, err
			}
			return pending.timing, pending.err
		case <-ctx.Done():
			s.mu.Lock()
			pending.waiters--
			if pending.waiters == 0 {
				pending.retiring = true
				pending.cancel()
			}
			s.mu.Unlock()
			return ProviderTiming{Queue: time.Since(started)}, ctx.Err()
		}
	}
}

func (s *ProviderScheduler) runLegacyPreparation(ctx context.Context, key legacyPreparationKey, pending *legacyPreparation, alive func() bool, load func(context.Context, string) error) {
	defer pending.cancel()
	release, timing, err := s.acquireLegacyModelDirect(ctx, key.gate, key.model, key.context, alive, load)
	if release != nil {
		release()
	}
	s.mu.Lock()
	pending.err, pending.timing = err, timing
	delete(s.legacyPreparing, key)
	close(pending.done)
	s.mu.Unlock()
}
