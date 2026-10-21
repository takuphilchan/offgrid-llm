package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestSchedulerOwnsEmbeddingResidencySwitchAndShutdown(t *testing.T) {
	registry := NewProviderRegistry()
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	var unloads atomic.Int32
	engine := NewEmbeddingEngine()
	engine.newImplementation = func() (EmbeddingImpl, error) {
		return &providerEmbeddingFixture{identity: "pinned", unloads: &unloads}, nil
	}
	if err := scheduler.AttachEmbeddingEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) { return false, nil }); err != nil {
		t.Fatal(err)
	}
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.EmbeddingUnmeasuredResidents != 1 || snapshot.LegacyUnmeasuredResidents != 0 {
		t.Fatalf("embedding residency was represented as free or llama memory: %+v", snapshot)
	}
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		if engine.IsLoaded() || unloads.Load() != 1 {
			return nil, errors.New("native worker opened on top of embedding runtime")
		}
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.EmbeddingUnmeasuredResidents != 0 {
		t.Fatalf("retired embedding remained resident: %+v", snapshot)
	}
	if err := scheduler.Close(); err != nil || unloads.Load() != 1 {
		t.Fatalf("scheduler shutdown failed after retirement: err=%v unloads=%d", err, unloads.Load())
	}
	if timing := scheduler.EmbeddingTimingSnapshot(); timing.Samples < 2 {
		t.Fatalf("embedding load/retirement was not accounted: %+v", timing)
	}
}

func TestSchedulerEmbeddingAttachmentOwnsFinalRetirement(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEmbeddingEngine()
	var unloads atomic.Int32
	engine.newImplementation = func() (EmbeddingImpl, error) {
		return &providerEmbeddingFixture{identity: "pinned", unloads: &unloads}, nil
	}
	if err := scheduler.AttachEmbeddingEngine(engine); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.AttachEmbeddingEngine(engine); err == nil {
		t.Fatal("duplicate ownership transfer was accepted")
	}
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Close(); err != nil || engine.IsLoaded() || unloads.Load() != 1 {
		t.Fatalf("scheduler did not retire attached embedding runtime: err=%v loaded=%v unloads=%d", err, engine.IsLoaded(), unloads.Load())
	}
	other, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := other.AttachEmbeddingEngine(engine); err == nil {
		t.Fatal("embedding owner was silently replaced")
	}
}
