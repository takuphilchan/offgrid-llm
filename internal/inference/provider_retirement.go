package inference

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Called only while the serialized load lease is held. Removal from the map
// precedes Close so a concurrent request cannot reuse a worker being unloaded.
func (s *ProviderScheduler) releaseIdleResidents(ctx context.Context, selected string) (time.Duration, time.Duration, error) {
	return s.releaseIdleResidentsMatching(ctx, func(key string) bool { return key != selected })
}

func (s *ProviderScheduler) releaseIdleResidentsMatching(ctx context.Context, retire func(string) bool) (time.Duration, time.Duration, error) {
	var retired []ProviderRuntime
	var waiting time.Duration
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return waiting, 0, errors.New("provider scheduler is closed")
		}
		if s.unloadErr != nil {
			err := s.unloadErr
			s.mu.Unlock()
			return waiting, 0, fmt.Errorf("provider retirement is unresolved: %w", err)
		}
		var retiring <-chan struct{}
		for key, pending := range s.loading {
			if pending.retiring && retire(key) {
				retiring = pending.done
				break
			}
		}
		if retiring != nil {
			s.mu.Unlock()
			start := time.Now()
			select {
			case <-retiring:
				waiting += time.Since(start)
				continue
			case <-ctx.Done():
				return waiting + time.Since(start), 0, ctx.Err()
			case <-s.closeCtx.Done():
				return waiting + time.Since(start), 0, errors.New("provider scheduler is closed")
			}
		}
		busy := false
		for key := range s.resident {
			if retire(key) && s.active[key] != 0 {
				busy = true
				break
			}
		}
		if !busy {
			for key, runtime := range s.resident {
				if retire(key) {
					retired = append(retired, runtime)
					delete(s.resident, key)
					delete(s.residentMemory, key)
				}
			}
			s.retiring += len(retired)
			s.mu.Unlock()
			break
		}
		changed := s.idleCh
		s.mu.Unlock()
		start := time.Now()
		select {
		case <-changed:
			waiting += time.Since(start)
		case <-ctx.Done():
			return waiting + time.Since(start), 0, ctx.Err()
		case <-s.closeCtx.Done():
			return waiting + time.Since(start), 0, errors.New("provider scheduler is closed")
		}
	}
	if len(retired) == 0 {
		return waiting, 0, nil
	}
	defer func() {
		s.mu.Lock()
		s.retiring -= len(retired)
		s.mu.Unlock()
	}()
	unloading := time.Now()
	var closeErr error
	for _, runtime := range retired {
		if err := runtime.Close(); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	if closeErr != nil {
		s.recordUnloadFailure(closeErr)
		return waiting, time.Since(unloading), fmt.Errorf("unload prior provider before switching: %w", closeErr)
	}
	return waiting, time.Since(unloading), nil
}

func (s *ProviderScheduler) recordUnloadFailure(err error) {
	s.mu.Lock()
	s.unloadErr = errors.Join(s.unloadErr, err)
	s.mu.Unlock()
}
