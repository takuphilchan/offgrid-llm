package inference

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ConfigureLegacyShutdown transfers final llama-process cleanup to the
// scheduler. It is registered once before any inference admission and runs
// only after provider leases drain.
func (s *ProviderScheduler) ConfigureLegacyShutdown(shutdown func() error) error {
	if shutdown == nil {
		return errors.New("legacy shutdown callback is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.legacyShutdown != nil || len(s.requests) != 0 || len(s.resident) != 0 || len(s.loading) != 0 || s.legacyRuns != 0 || s.nativeRun {
		return errors.New("legacy shutdown can only be configured before provider activity")
	}
	s.legacyShutdown = shutdown
	return nil
}

// ConfigureLegacyResidency gives diagnostics a truthful count of llama cache
// processes without pretending their CPU/GPU memory usage is measured. The
// observer must be safe to call concurrently and must not call the scheduler.
func (s *ProviderScheduler) ConfigureLegacyResidency(resident func() int) error {
	if resident == nil {
		return errors.New("legacy residency observer is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.legacyResident != nil || len(s.requests) != 0 || len(s.resident) != 0 || len(s.loading) != 0 || s.legacyRuns != 0 || s.nativeRun {
		return errors.New("legacy residency can only be configured before provider activity")
	}
	s.legacyResident = resident
	return nil
}

// AttachLlamaCache transfers the existing llama process lifecycle to the
// scheduler in one startup-only operation. ModelCache remains the GGUF process
// implementation, but restart admission, residency reporting, and final
// retirement cannot be registered as a partially configured graph.
func (s *ProviderScheduler) AttachLlamaCache(cache *ModelCache, engine Engine) (*LlamaCPPProcess, error) {
	if cache == nil || engine == nil {
		return nil, errors.New("llama cache and engine are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.legacyResident != nil || s.legacyMemory != nil || s.legacyShutdown != nil || s.legacyCache != nil || len(s.requests) != 0 || len(s.resident) != 0 || len(s.loading) != 0 || s.legacyRuns != 0 || s.nativeRun {
		return nil, errors.New("llama process owner can only be attached before provider activity")
	}
	if len(cache.LoadedModelIDs()) != 0 || engine.IsLoaded() {
		return nil, errors.New("llama process owner requires an idle cache and engine")
	}
	process := NewLlamaCPPProcess(cache, engine)
	process.owner = s
	cache.SetRestartAdmission(func(ctx context.Context) (func(), error) {
		release, _, err := s.AcquireLegacyMaintenance(ctx)
		return release, err
	})
	s.legacyResident = func() int { return len(cache.LoadedModelIDs()) }
	s.legacyMemory = cache.ProcessMemorySnapshot
	s.legacyCache, s.legacyEngine = cache, engine
	s.legacyShutdown = func() error {
		cache.StopMonitor()
		return s.withLegacyProcessLease(context.Background(), func(ctx context.Context) error {
			_, err := process.RetireAll(ctx)
			return err
		})
	}
	return process, nil
}

// Close cancels admission first, waits for startup and generation owners to
// settle, then retires resident workers. It never reports a clean shutdown
// while a provider load can still publish or reject a child process.
func (s *ProviderScheduler) Close() error {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		s.mu.Lock()
		err := s.closeErr
		s.mu.Unlock()
		return err
	}
	s.closed = true
	s.closeRun()
	pending := make([]<-chan struct{}, 0, len(s.loading)+len(s.requests))
	for _, load := range s.loading {
		pending = append(pending, load.done)
	}
	for _, request := range s.requests {
		if request.cancelAt.IsZero() {
			request.cancelAt = time.Now()
		}
		request.cancel()
		pending = append(pending, request.done)
	}
	runtimes := make([]ProviderRuntime, 0, len(s.resident))
	legacyShutdown := s.legacyShutdown
	embeddingOwner := s.embeddingOwner
	for key, runtime := range s.resident {
		runtimes = append(runtimes, runtime)
		delete(s.resident, key)
		delete(s.residentMemory, key)
	}
	clear(s.residentMemory)
	s.mu.Unlock()

	var result error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			result = errors.Join(result, fmt.Errorf("provider shutdown still has in-flight work: %w", ctx.Err()))
			goto retire
		}
	}
	for {
		s.mu.Lock()
		activeCancel := len(s.pendingCancel) != 0
		changed := s.cancelCh
		s.mu.Unlock()
		if !activeCancel {
			break
		}
		select {
		case <-changed:
		case <-ctx.Done():
			result = errors.Join(result, fmt.Errorf("provider shutdown still has an in-flight cancellation signal: %w", ctx.Err()))
			goto retire
		}
	}
	for {
		s.mu.Lock()
		active := len(s.active) != 0
		changed := s.idleCh
		s.mu.Unlock()
		if !active {
			break
		}
		select {
		case <-changed:
		case <-ctx.Done():
			result = errors.Join(result, fmt.Errorf("provider shutdown still has an active runtime check: %w", ctx.Err()))
			goto retire
		}
	}
	// A cancelled request still owns its runtime until Generate has returned.
	// Closing the worker earlier races its stream and can lose the terminal
	// effect state. If cancellation did not settle within the bound, Close below
	// becomes a forced retirement and the outcome remains explicitly uncertain.
retire:
	for _, runtime := range runtimes {
		result = errors.Join(result, runtime.Close())
	}
	if result != nil {
		goto finished
	}
	for {
		s.mu.Lock()
		active := s.legacyRuns != 0 || s.nativeRun
		changed := s.familyCh
		s.mu.Unlock()
		if !active {
			break
		}
		select {
		case <-changed:
		case <-ctx.Done():
			result = errors.Join(result, fmt.Errorf("provider shutdown still has an active model lease: %w", ctx.Err()))
			goto finished
		}
	}
	if result == nil {
		if embeddingOwner != nil {
			result = errors.Join(result, embeddingOwner.Unload())
		}
		if legacyShutdown != nil {
			result = errors.Join(result, legacyShutdown())
		}
	}
finished:
	s.mu.Lock()
	result = errors.Join(result, s.unloadErr)
	s.closeErr = result
	close(s.closeDone)
	s.mu.Unlock()
	return result
}
