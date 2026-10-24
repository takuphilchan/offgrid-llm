package inference

import (
	"context"
	"errors"
	"fmt"
)

// llamaProcessCache is the established llama-server process owner. Keeping the
// narrow interface here lets the provider lifecycle be tested without opening
// a real model or changing ModelCache's CPU/GPU/projector behavior.
type llamaProcessCache interface {
	SetContextSize(int)
	GetOrLoadContext(context.Context, string, string, string) (*ModelInstance, error)
	IsModelAlive(string) bool
	InstanceGeneration(string) uint64
	LoadedModelIDs() []string
	Unload(string) error
}

// LlamaProcessLifecycle is the compatibility load/retire surface consumed by
// the service while GGUF generation is migrated to ProviderRuntime events.
type LlamaProcessLifecycle interface {
	Activate(context.Context, string, string, string, LoadOptions) (int, error)
	Alive(string) bool
	Generation(string) uint64
	RetireAll(context.Context) (bool, error)
}

// LlamaCPPProcess adapts the existing binary cache and HTTP engine as one
// lifecycle. The shared scheduler still owns admission; callers must hold its
// generation or exclusive maintenance lease before activating or retiring it.
type LlamaCPPProcess struct {
	cache  llamaProcessCache
	engine Engine
	owner  *ProviderScheduler
}

func NewLlamaCPPProcess(cache *ModelCache, engine Engine) *LlamaCPPProcess {
	p := &LlamaCPPProcess{engine: engine}
	if cache != nil {
		p.cache = cache
	}
	return p
}

func (p *LlamaCPPProcess) Alive(modelID string) bool {
	return p != nil && p.cache != nil && p.cache.IsModelAlive(modelID)
}

func (p *LlamaCPPProcess) Generation(modelID string) uint64 {
	if p == nil || p.cache == nil {
		return 0
	}
	return p.cache.InstanceGeneration(modelID)
}

// Activate preserves the previous cache-load -> port-selection -> engine-load
// ordering. In particular, the context is set before llama-server starts and
// the projector path is passed to the same existing cache implementation.
func (p *LlamaCPPProcess) Activate(ctx context.Context, modelID, modelPath, projectorPath string, opts LoadOptions) (int, error) {
	if p == nil || p.cache == nil || p.engine == nil {
		return 0, errors.New("llama process lifecycle is unavailable")
	}
	if !p.admitted(ctx) {
		return 0, errors.New("llama activation requires provider scheduler admission")
	}
	if modelID == "" || modelPath == "" || opts.ContextSize < 1 {
		return 0, errors.New("llama activation requires model identity, path, and context")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	p.cache.SetContextSize(opts.ContextSize)
	instance, err := p.cache.GetOrLoadContext(ctx, modelID, modelPath, projectorPath)
	if err != nil {
		return 0, fmt.Errorf("failed to load model: %w", err)
	}
	if instance == nil || instance.Port < 1 {
		return 0, errors.New("llama cache returned no usable process")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if setter, ok := p.engine.(interface{ SetPort(int) }); ok {
		setter.SetPort(instance.Port)
	}
	if err := p.engine.Load(ctx, modelPath, opts); err != nil {
		return 0, fmt.Errorf("connect inference engine: %w", err)
	}
	// Confirm the actual per-slot allocation, not just a ready process/port.
	if reporter, ok := p.engine.(interface {
		ToolRuntimeInfo(context.Context) (*ToolRuntimeInfo, error)
	}); ok {
		info, err := reporter.ToolRuntimeInfo(ctx)
		if err != nil || info == nil || info.Context < opts.ContextSize {
			_ = p.engine.Unload()
			_ = p.cache.Unload(modelID)
			if err != nil {
				return 0, fmt.Errorf("confirm runtime context: %w", err)
			}
			return 0, fmt.Errorf("runtime did not confirm the requested %d-token context; no generation dispatched", opts.ContextSize)
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return instance.Port, nil
}

// RetireAll stops every cache-owned llama child and disconnects the engine.
// A partial failure is returned to the scheduler so it can latch unresolved
// residency instead of loading another provider on unmeasured hardware.
func (p *LlamaCPPProcess) RetireAll(ctx context.Context) (bool, error) {
	if p == nil {
		return false, nil
	}
	if !p.admitted(ctx) {
		return false, errors.New("llama retirement requires provider scheduler admission")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	retired := false
	if p.cache != nil {
		for _, id := range p.cache.LoadedModelIDs() {
			if err := ctx.Err(); err != nil {
				return retired, err
			}
			retired = true
			if err := p.cache.Unload(id); err != nil {
				return retired, fmt.Errorf("unload llama model %q: %w", id, err)
			}
		}
		if len(p.cache.LoadedModelIDs()) != 0 {
			return true, errors.New("llama model cache remains resident after retirement")
		}
	}
	if p.engine != nil && p.engine.IsLoaded() {
		retired = true
		if err := p.engine.Unload(); err != nil {
			return retired, fmt.Errorf("disconnect llama engine: %w", err)
		}
	}
	return retired, nil
}

func (p *LlamaCPPProcess) admitted(ctx context.Context) bool {
	if p.owner == nil {
		return true // Standalone compatibility fixtures own their lifecycle.
	}
	lease, ok := ctx.Value(legacyProcessLeaseKey{}).(*legacyProcessLease)
	return ok && lease.owner == p.owner && lease.active.Load()
}
