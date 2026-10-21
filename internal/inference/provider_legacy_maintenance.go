package inference

import (
	"context"
	"errors"
	"time"
)

// LegacyMaintenance exposes only the cache mutations needed by service model
// operations. The scheduler constructs it under an exclusive maintenance
// lease; callers cannot obtain it from a model package or provider worker.
type LegacyMaintenance struct {
	ctx    context.Context
	cache  *ModelCache
	engine Engine
	timing *ProviderTiming
}

func (m LegacyMaintenance) HotSwap(modelID, modelPath, projectorPath string) (*HotSwapResult, error) {
	if err := m.ctx.Err(); err != nil {
		return nil, err
	}
	started := time.Now()
	result, err := m.cache.HotSwapContext(m.ctx, modelID, modelPath, projectorPath)
	if result == nil || result.Method != "preloaded" {
		m.timing.Load += time.Since(started)
	}
	return result, err
}

func (m LegacyMaintenance) UnloadModel(modelID string) error {
	if err := m.ctx.Err(); err != nil {
		return err
	}
	if !m.cache.IsModelAlive(modelID) {
		return nil
	}
	started := time.Now()
	err := m.cache.Unload(modelID)
	m.timing.Unload += time.Since(started)
	return err
}

func (m LegacyMaintenance) DisconnectEngine() error {
	if err := m.ctx.Err(); err != nil {
		return err
	}
	if m.engine.IsLoaded() {
		started := time.Now()
		err := m.engine.Unload()
		m.timing.Unload += time.Since(started)
		return err
	}
	return nil
}

// WithLegacyMaintenance moves service-initiated llama process mutations
// behind the same scheduler that gates generation and native workers. The
// callback keeps the exclusive lease through any registry/knowledge update
// that depends on the process result.
func (s *ProviderScheduler) WithLegacyMaintenance(ctx context.Context, action func(LegacyMaintenance) error) (ProviderTiming, error) {
	var timing ProviderTiming
	if action == nil {
		return timing, errors.New("legacy maintenance action is required")
	}
	release, timing, err := s.AcquireLegacyMaintenance(ctx)
	if err != nil {
		return timing, err
	}
	defer release()
	s.mu.Lock()
	cache, engine, closed := s.legacyCache, s.legacyEngine, s.closed
	s.mu.Unlock()
	if cache == nil || engine == nil || closed {
		return timing, errors.New("attached llama process owner is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return timing, err
	}
	actionErr := action(LegacyMaintenance{ctx: ctx, cache: cache, engine: engine, timing: &timing})
	return timing, actionErr
}
