package inference

import (
	"context"
	"errors"
	"fmt"
)

// checkResidentLocked enters with the scheduler lock and returns without it.
// retry means the caller may re-read scheduler state rather than reuse this
// runtime. The health pin covers both provider transport calls and prevents a
// concurrent model switch from retiring their process.
func (s *ProviderScheduler) checkResidentLocked(ctx context.Context, descriptor ProviderDescriptor, key string, runtime ProviderRuntime, reserve bool, replacements *int) (ProviderRuntime, bool, error) {
	s.active[key]++
	s.mu.Unlock()
	checkCtx, cancelCheck := context.WithCancel(ctx)
	stopCheck := context.AfterFunc(s.closeCtx, cancelCheck)
	resources, healthErr := ProviderResources{}, runtime.Health(checkCtx)
	if healthErr == nil {
		resources, healthErr = runtime.Resources(checkCtx)
		if healthErr == nil {
			healthErr = resources.Validate(descriptor)
		}
	}
	stopCheck()
	cancelCheck()
	if healthErr != nil {
		if ctx.Err() != nil {
			s.mu.Lock()
			s.releaseActiveLocked(key)
			s.mu.Unlock()
			return nil, false, ctx.Err()
		}
		s.mu.Lock()
		s.releaseActiveLocked(key)
		if s.closed {
			s.mu.Unlock()
			return nil, false, errors.New("provider scheduler is closed")
		}
		owned := s.resident[key] == runtime && s.active[key] == 0
		busy := s.resident[key] == runtime && s.active[key] != 0
		var retiring *providerLoad
		if owned {
			delete(s.resident, key)
			delete(s.residentMemory, key)
			// Keep cleanup visible to Close and to another request for the
			// same model. Failed health is not proof of process exit.
			retiring = &providerLoad{done: make(chan struct{}), retiring: true}
			s.loading[key] = retiring
		}
		s.mu.Unlock()
		if busy {
			return nil, false, fmt.Errorf("resident provider is active and unhealthy: %w", healthErr)
		}
		if owned {
			closeErr := runtime.Close()
			s.mu.Lock()
			retiring.err = closeErr
			if closeErr != nil {
				s.unloadErr = errors.Join(s.unloadErr, closeErr)
			}
			delete(s.loading, key)
			close(retiring.done)
			s.mu.Unlock()
			if closeErr != nil {
				return nil, false, fmt.Errorf("unhealthy provider could not be retired: %w", closeErr)
			}
			*replacements = *replacements + 1
		}
		if *replacements > 1 {
			return nil, false, fmt.Errorf("resident provider unavailable after bounded replacement: %w", healthErr)
		}
		return nil, true, nil
	}
	s.mu.Lock()
	if s.closed {
		s.releaseActiveLocked(key)
		s.mu.Unlock()
		return nil, false, errors.New("provider scheduler is closed")
	}
	if s.resident[key] != runtime {
		s.releaseActiveLocked(key)
		s.mu.Unlock()
		return nil, true, nil
	}
	if err := s.accountResidentMemoryLocked(key, resources); err != nil {
		s.releaseActiveLocked(key)
		s.mu.Unlock()
		return nil, false, err
	}
	if !reserve {
		s.releaseActiveLocked(key)
	}
	s.mu.Unlock()
	return runtime, false, nil
}

// releaseActiveLocked wakes a model switch or shutdown waiting for the last
// transport call against a resident worker to complete.
func (s *ProviderScheduler) releaseActiveLocked(key string) {
	s.active[key]--
	if s.active[key] == 0 {
		delete(s.active, key)
		close(s.idleCh)
		s.idleCh = make(chan struct{})
	}
}
