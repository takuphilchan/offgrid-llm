package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type failedEmbeddingLoad struct {
	loadErr    error
	dimensions int
	unloads    atomic.Int32
	stopOK     bool
}

type uncertainGenerationEmbedding struct{ failedEmbeddingLoad }

func (*uncertainGenerationEmbedding) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errProviderStopUncertain
}

func TestEmbeddingGenerationUncertainRetirementImmediatelyPoisonsAdmission(t *testing.T) {
	engine := NewEmbeddingEngine()
	fixture := &uncertainGenerationEmbedding{failedEmbeddingLoad: failedEmbeddingLoad{dimensions: 2}}
	engine.newImplementation = func() (EmbeddingImpl, error) { return fixture, nil }
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.AttachEmbeddingEngine(engine); err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if err := engine.Load(context.Background(), "fixture", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.GenerateEmbeddings(context.Background(), &api.EmbeddingRequest{Model: "fixture", Input: "hello"}); !errors.Is(err, errProviderStopUncertain) {
		t.Fatalf("retirement error lost: %v", err)
	}
	if engine.IsLoaded() || engine.implementation != fixture || !scheduler.MemorySnapshot().Unresolved {
		t.Fatal("generation retirement uncertainty remained ready/free")
	}
	if release, _, err := scheduler.AcquireLegacyMaintenance(context.Background()); err == nil || release != nil {
		t.Fatal("uncertain generation allowed another allocation")
	}
}

func (f *failedEmbeddingLoad) Load(context.Context, string, EmbeddingOptions) error { return f.loadErr }
func (f *failedEmbeddingLoad) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("failed runtime must not generate")
}
func (f *failedEmbeddingLoad) GetDimensions() int { return f.dimensions }
func (f *failedEmbeddingLoad) Unload() error {
	f.unloads.Add(1)
	if f.stopOK {
		return nil
	}
	return errors.New("isolated retirement uncertainty fixture")
}

func TestEmbeddingFailedLoadSuccessfulCleanupRequiresExplicitReload(t *testing.T) {
	for _, parked := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "parked"}[parked], func(t *testing.T) {
			engine := NewEmbeddingEngine()
			fixture := &failedEmbeddingLoad{loadErr: errors.New("failed reload"), dimensions: 2, stopOK: true}
			calls := 0
			engine.newImplementation = func() (EmbeddingImpl, error) { calls++; return fixture, nil }
			if parked {
				engine.parked = true
				engine.modelPath = "original"
				engine.expectedIdentity = "original-identity"
				engine.loadOptions = DefaultEmbeddingOptions()
			}
			var err error
			if parked {
				_, err = engine.GenerateEmbeddingsForIdentity(context.Background(), &api.EmbeddingRequest{Model: "original", Input: "text"}, "original-identity")
			} else {
				err = engine.Load(context.Background(), "original", DefaultEmbeddingOptions())
			}
			if !errors.Is(err, fixture.loadErr) || engine.IsLoaded() || engine.implementation != nil || engine.resident.Load() || engine.retirementErr != nil || engine.parked || engine.expectedIdentity != "" || engine.modelPath != "" {
				t.Fatal("failed load cleanup left stale activation/reload state", err)
			}
			if _, err := engine.GenerateEmbeddingsForIdentity(context.Background(), &api.EmbeddingRequest{Model: "original", Input: "text"}, "original-identity"); err == nil || calls != 1 {
				t.Fatal("failed parked load retried automatically or used empty path")
			}
			fixture.loadErr = nil
			if err := engine.Load(context.Background(), "original", DefaultEmbeddingOptions()); err != nil || calls != 2 || !engine.IsLoaded() {
				t.Fatal("confirmed cleanup blocked explicit reload", err)
			}
			if err := engine.Unload(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEmbeddingFactoryErrorRetainsReturnedImplementation(t *testing.T) {
	engine := NewEmbeddingEngine()
	fixture := &failedEmbeddingLoad{}
	factoryErr := errors.New("partial factory initialization failed")
	engine.newImplementation = func() (EmbeddingImpl, error) { return fixture, factoryErr }
	if err := engine.Load(context.Background(), "fixture", DefaultEmbeddingOptions()); !errors.Is(err, factoryErr) || !errors.Is(err, errProviderStopUncertain) || engine.implementation != fixture || engine.IsLoaded() {
		t.Fatal("partial factory ownership disappeared", err)
	}
}

func TestEmbeddingFailedLoadRetainsUncertainOwnershipAndBlocksAllAdmission(t *testing.T) {
	for _, failure := range []string{"load", "dimensions"} {
		t.Run(failure, func(t *testing.T) {
			registry := NewProviderRegistry()
			scheduler, err := NewProviderScheduler(registry, 1)
			if err != nil {
				t.Fatal(err)
			}
			fixture := &failedEmbeddingLoad{}
			if failure == "load" {
				fixture.loadErr = errors.New("original load failure")
				fixture.dimensions = 2
			}
			engine := NewEmbeddingEngine()
			var factories atomic.Int32
			engine.newImplementation = func() (EmbeddingImpl, error) { factories.Add(1); return fixture, nil }
			if err := scheduler.AttachEmbeddingEngine(engine); err != nil {
				t.Fatal(err)
			}
			if err := engine.Load(context.Background(), "fixture", DefaultEmbeddingOptions()); err == nil || !errors.Is(err, errProviderStopUncertain) || fixture.loadErr != nil && !errors.Is(err, fixture.loadErr) {
				t.Fatalf("lost load/cleanup error: %v", err)
			}
			if engine.IsLoaded() || engine.implementation != fixture {
				t.Fatal("failed runtime lost ownership or claimed ready")
			}
			sticky := engine.retirementErr
			for range 4 {
				if err := engine.Unload(); err != sticky {
					t.Fatal("repeat cleanup lost or repeatedly expanded original uncertainty", err)
				}
			}
			if memory := scheduler.MemorySnapshot(); !memory.Unresolved || memory.EmbeddingUnmeasuredResidents != 1 {
				t.Fatalf("uncertain embedding appeared free: %+v", memory)
			}
			if err := engine.Load(context.Background(), "second", DefaultEmbeddingOptions()); err == nil || factories.Load() != 1 {
				t.Fatal("second embedding load bypassed unresolved resources")
			}
			if _, err := engine.GenerateEmbeddings(context.Background(), &api.EmbeddingRequest{Model: "fixture", Input: "hello"}); err == nil {
				t.Fatal("failed runtime generated embeddings")
			}
			if release, _, err := scheduler.AcquireLegacyMaintenance(context.Background()); err == nil || release != nil {
				t.Fatal("legacy allocation reused unresolved resources")
			}
			descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:fixture", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
			var native atomic.Int32
			if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
				native.Add(1)
				return &schedulerFake{descriptor: descriptor}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := scheduler.Runtime(context.Background(), descriptor); err == nil || native.Load() != 0 {
				t.Fatal("native allocation reused unresolved resources")
			}
			before := fixture.unloads.Load()
			if err := scheduler.Close(); err == nil || fixture.unloads.Load() <= before {
				t.Fatal("final owner did not retry tracked cleanup or hid failure")
			}
		})
	}
}
