package app

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cache"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/stats"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// core owns the low-level workspace and inference graph. Higher-level services
// are attached to the same cleanup stack before publishing a Runtime.
type core struct {
	owned             cleanupStack
	workspace         string
	registry          *models.Registry
	engine            inference.Engine
	embedding         *inference.EmbeddingEngine
	monitor           *resource.Monitor
	resources         *resource.SystemResources
	stats             *stats.Tracker
	cache             *cache.ResponseCache
	modelCache        *inference.ModelCache
	llamaProcess      inference.LlamaProcessLifecycle
	providerRegistry  *inference.ProviderRegistry
	providerScheduler *inference.ProviderScheduler
}

// newCore mirrors the legacy acquisition order. A failed acquisition releases
// everything already owned before returning an error; it never hands out a
// partially usable graph.
func newCore(cfg *config.Config) (_ *core, err error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	c := &core{}
	defer func() {
		if err != nil {
			err = c.owned.rollback(err)
		}
	}()
	owner, err := storage.AcquireOwnership(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	c.owned.add("workspace ownership", owner.Close)
	if err := storage.PrepareDataLayout(cfg.ModelsDir, cfg.DataDir); err != nil {
		return nil, fmt.Errorf("prepare workspace: %w", err)
	}
	c.workspace, err = owner.WorkspaceID()
	if err != nil {
		return nil, fmt.Errorf("workspace identity: %w", err)
	}
	c.registry = models.NewRegistry(cfg.ModelsDir)
	if cfg.UseMockEngine {
		log.Println("Using mock engine (no real inference)")
		c.engine = inference.NewMockEngine()
	} else {
		log.Println("Using llama.cpp engine")
		c.engine = inference.NewLlamaEngine()
	}
	c.monitor = resource.NewMonitor(5 * time.Second)
	c.stats = stats.NewTracker()
	c.cache = cache.NewResponseCache(1000, time.Hour)
	c.cache.StartCleanupRoutine(15 * time.Minute)
	c.owned.add("response cache", func() error { c.cache.StopCleanupRoutine(); return nil })
	c.embedding = inference.NewEmbeddingEngine(filepath.Join(cfg.ModelsDir, "..", "bin"))
	if scanErr := c.registry.ScanModels(); scanErr != nil {
		log.Printf("Warning: Failed to scan models: %v", scanErr)
	}
	c.monitor.Start()
	c.owned.add("resource monitor", func() error { c.monitor.Stop(); return nil })
	return c, nil
}

// The legacy graph creates the provider scheduler only after session recovery.
func (c *core) initProviderScheduler(cfg *config.Config) error {
	c.providerRegistry = inference.NewProviderRegistry()
	var err error
	c.providerScheduler, err = inference.NewProviderScheduler(c.providerRegistry, cfg.InferenceSlots)
	if err != nil {
		return fmt.Errorf("initialize provider scheduler: %w", err)
	}
	c.owned.addBarrier("provider scheduler", c.providerScheduler.Close)
	if err := c.providerScheduler.AttachEmbeddingEngine(c.embedding); err != nil {
		return fmt.Errorf("transfer embedding engine ownership to provider scheduler: %w", err)
	}
	return nil
}

// Model cache allocation remains after optional service construction, exactly
// where the legacy constructor configured it.
func (c *core) initModelCache(cfg *config.Config) error {
	c.resources, _ = resource.DetectResources()
	c.modelCache = inference.NewConfiguredModelCache(cfg, c.resources)
	if c.providerScheduler == nil {
		c.modelCache.StopMonitor()
		c.modelCache.UnloadAll()
		return fmt.Errorf("model cache requires provider scheduler ownership")
	}
	process, err := c.providerScheduler.AttachLlamaCache(c.modelCache, c.engine)
	if err != nil {
		c.modelCache.StopMonitor()
		c.modelCache.UnloadAll()
		return fmt.Errorf("transfer model cache ownership to provider scheduler: %w", err)
	}
	c.llamaProcess = process
	return nil
}
