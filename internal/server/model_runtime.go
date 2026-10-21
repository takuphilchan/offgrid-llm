package server

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// startLlamaServer is deprecated - models are now loaded on-demand via cache
// However, we use this opportunity to pre-warm models into the OS page cache
func (s *Server) startLlamaServer() error {
	// Start background pre-warming of all models into OS page cache (if enabled)
	// This dramatically speeds up model switching (from 60-120s to 5-15s)
	if s.config.PrewarmModels || s.config.FastSwitchMode {
		log.Println("Starting background model pre-warming for faster switching...")
		s.modelCache.PrewarmAllModels()
	} else {
		log.Println("Model pre-warming disabled (set OFFGRID_PREWARM_MODELS=true to enable)")
	}

	return nil
}

// switchModel uses the model cache to load or switch to a model
// Returns the port of the llama-server instance running this model
func (s *Server) switchModel(modelID string) error {
	return s.switchModelContext(context.Background(), modelID)
}

func (s *Server) switchModelContext(ctx context.Context, modelID string) error {
	return s.switchModelWithContextSize(ctx, modelID, s.modelContextWindow(modelID))
}

func (s *Server) switchModelWithContextSize(ctx context.Context, modelID string, effectiveContext int) error {
	s.modelMutex.Lock()
	defer s.modelMutex.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	metadata, err := s.registry.GetChatModel(modelID)
	if err != nil {
		return err
	}

	process := s.llamaProcess
	if process == nil {
		if s.providerScheduler != nil {
			return errors.New("scheduler-owned llama process is unavailable")
		}
		process = inference.NewLlamaCPPProcess(s.modelCache, s.engine)
	}
	// A cache-owned child may restart at the same port. An alive process is
	// insufficient: the HTTP engine must be connected to this generation.
	if s.currentModelID == modelID && s.currentPort > 0 && s.currentContext == effectiveContext {
		generation := process.Generation(modelID)
		if generation != 0 && s.currentGeneration == generation && process.Alive(modelID) {
			log.Printf("Model %s already loaded on port %d, skipping reload", modelID, s.currentPort)
			_ = s.registry.LoadModel(modelID)
			return nil
		}
		log.Printf("Model %s process changed or stopped; reconnecting...", modelID)
	}

	log.Printf("Switching to model: %s", modelID)

	loadOptions := inference.DefaultLoadOptions()
	loadOptions.NumThreads = s.config.NumThreads
	loadOptions.ContextSize = effectiveContext
	port, err := process.Activate(ctx, modelID, metadata.Path, metadata.ProjectorPath, loadOptions)
	if err != nil {
		// The engine port may have changed before a failed connect. Never let
		// the next request fast-path the old model against an uncertain port.
		s.currentModelID = ""
		s.currentContext = 0
		s.currentPort = 0
		s.currentGeneration = 0
		return err
	}
	// Do not mark the profile active until the engine has connected.
	s.currentModelID = modelID
	s.currentContext = effectiveContext
	s.currentPort = port
	s.currentGeneration = process.Generation(modelID)
	// Registry state is committed only after the runtime is reachable.
	if err := s.registry.LoadModel(modelID); err != nil {
		s.currentModelID = ""
		s.currentContext = 0
		s.currentPort = 0
		s.currentGeneration = 0
		return err
	}
	s.syncRegistryLoadState()

	log.Printf("Now using model %s on port %d", modelID, port)
	return nil
}

func (s *Server) syncRegistryLoadState() {
	if s.modelCache == nil || s.registry == nil {
		return
	}
	loaded := make(map[string]bool)
	for _, id := range s.modelCache.LoadedModelIDs() {
		loaded[id] = true
	}
	for _, model := range s.registry.ListModels() {
		_ = s.registry.SetLoaded(model.ID, loaded[model.ID])
	}
}

func (s *Server) acquireInference(ctx context.Context, modelID string) (func(), error) {
	return s.acquireInferenceContext(ctx, modelID, s.modelContextWindow(modelID), nil)
}

func (s *Server) acquireInferenceContext(ctx context.Context, modelID string, contextWindow int, loading func() error) (func(), error) {
	release, _, err := s.acquireInferenceContextTimed(ctx, modelID, contextWindow, loading)
	return release, err
}

func (s *Server) acquireInferenceLease(ctx context.Context, modelID string, contextWindow int, loading func() error) (*inference.LegacyGenerationLease, error) {
	release, timing, err := s.acquireInferenceContextTimed(ctx, modelID, contextWindow, loading)
	if err != nil {
		return nil, err
	}
	var record func(inference.ProviderTiming)
	if s.providerScheduler != nil {
		record = s.providerScheduler.RecordLegacyTiming
	}
	lease, err := inference.NewLegacyGenerationLease(release, timing, record)
	if err != nil {
		release()
	}
	return lease, err
}

// llamaBindingCurrent checks both process health and the generation to which
// the service engine was last attached. The monitor may replace a child while
// preserving its model ID and even its loopback port.
func (s *Server) llamaBindingCurrent(modelID string) bool {
	process := s.llamaProcess
	if process == nil {
		if s.providerScheduler != nil {
			return false
		}
		process = inference.NewLlamaCPPProcess(s.modelCache, s.engine)
	}
	if !process.Alive(modelID) {
		return false
	}
	generation := process.Generation(modelID)
	s.modelMutex.Lock()
	current := generation != 0 && s.currentModelID == modelID && s.currentPort > 0 && s.currentGeneration == generation
	s.modelMutex.Unlock()
	return current
}

func (s *Server) acquireInferenceContextTimed(ctx context.Context, modelID string, contextWindow int, loading func() error) (func(), inference.ProviderTiming, error) {
	var timing inference.ProviderTiming
	// The shared scheduler coordinates admission and the established llama
	// lifecycle lease through the caller's generation. The model cache still
	// owns the actual llama process during this compatibility migration.
	var loadingElapsed time.Duration
	switcher := func(ctx context.Context, model string) error {
		loadCtx := ctx
		cancel := func() {}
		if s.config != nil && s.config.ModelLoadTimeout > 0 {
			loadCtx, cancel = context.WithTimeout(ctx, time.Duration(s.config.ModelLoadTimeout)*time.Second)
		}
		defer cancel()
		started := time.Now()
		defer func() { loadingElapsed += time.Since(started) }()
		if loading != nil {
			if err := loading(); err != nil {
				return err
			}
		}
		return s.switchModelWithContextSize(loadCtx, model, contextWindow)
	}
	if s.providerScheduler != nil {
		if s.inferenceLifecycle == nil {
			return nil, timing, errors.New("inference lifecycle is unavailable")
		}
		var bound func() bool
		if s.llamaProcess != nil || s.modelCache != nil {
			bound = func() bool { return s.llamaBindingCurrent(modelID) }
		}
		return s.providerScheduler.AcquireLegacyModel(ctx, s.inferenceLifecycle, modelID, contextWindow, bound, switcher)
	}
	if s.inferenceLifecycle != nil && (s.llamaProcess != nil || s.modelCache != nil) && !s.llamaBindingCurrent(modelID) {
		s.inferenceLifecycle.Invalidate(modelID)
	}
	if s.inferenceLifecycle == nil {
		if err := switcher(ctx, modelID); err != nil {
			timing.Load = loadingElapsed
			return nil, timing, err
		}
		timing.Load = loadingElapsed
		return func() {}, timing, nil
	}
	waiting := time.Now()
	releaseLegacy, err := s.inferenceLifecycle.AcquireWithContext(ctx, modelID, contextWindow, switcher)
	timing.Load = loadingElapsed
	timing.Queue = time.Since(waiting) - loadingElapsed
	if err != nil {
		return nil, timing, err
	}
	return releaseLegacy, timing, nil
}
