package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type providerEmbeddingFixture struct {
	identity string
	started  chan struct{}
	resume   chan struct{}
	unloads  *atomic.Int32
}

type lateEmbeddingFixture struct {
	loadStarted chan struct{}
	loadFinish  chan struct{}
	embedStart  chan struct{}
	embedFinish chan struct{}
	unloads     atomic.Int32
}

func (f *lateEmbeddingFixture) Load(context.Context, string, EmbeddingOptions) error {
	if f.loadStarted != nil {
		close(f.loadStarted)
		<-f.loadFinish // Deliberately ignores cancellation.
	}
	return nil
}
func (f *lateEmbeddingFixture) Embed(context.Context, []string) ([][]float32, error) {
	close(f.embedStart)
	<-f.embedFinish // Deliberately ignores cancellation.
	return [][]float32{{1, 0}}, nil
}
func (f *lateEmbeddingFixture) Unload() error {
	f.unloads.Add(1)
	return nil
}
func (*lateEmbeddingFixture) GetDimensions() int { return 2 }

func TestEmbeddingLateLoadCannotBecomeSuccessfulActivation(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	fixture := &lateEmbeddingFixture{loadStarted: make(chan struct{}), loadFinish: make(chan struct{})}
	engine := NewEmbeddingEngine()
	engine.SetProviderScheduler(scheduler)
	engine.newImplementation = func() (EmbeddingImpl, error) { return fixture, nil }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- engine.Load(ctx, "model.gguf", DefaultEmbeddingOptions()) }()
	<-fixture.loadStarted
	snapshotDone := make(chan ProviderMemorySnapshot, 1)
	go func() { snapshotDone <- scheduler.MemorySnapshot() }()
	select {
	case snapshot := <-snapshotDone:
		if !snapshot.Unresolved || snapshot.EmbeddingUnmeasuredResidents != 0 {
			t.Fatalf("in-flight embedding load appeared settled: %+v", snapshot)
		}
	case <-time.After(250 * time.Millisecond):
		cancel()
		close(fixture.loadFinish)
		<-done
		t.Fatal("memory snapshot blocked behind an in-flight embedding load")
	}
	cancel()
	time.Sleep(2 * time.Millisecond)
	close(fixture.loadFinish)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("late load was reported successful: %v", err)
	}
	if !engine.IsLoaded() {
		t.Fatal("cancelled load lost a live runtime that still needs retirement")
	}
	if retired, err := engine.RetireForProvider(); err != nil || !retired || fixture.unloads.Load() != 1 {
		t.Fatalf("late load was not safely retired: retired=%v err=%v unloads=%d", retired, err, fixture.unloads.Load())
	}
	if timing := scheduler.EmbeddingTimingSnapshot(); timing.Samples != 2 || timing.Cancel <= 0 {
		t.Fatalf("cancelled load timing was not recorded: %+v", timing)
	}
}

func TestEmbeddingLateVectorsCannotCrossStop(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	fixture := &lateEmbeddingFixture{embedStart: make(chan struct{}), embedFinish: make(chan struct{})}
	engine := NewEmbeddingEngine()
	engine.SetProviderScheduler(scheduler)
	engine.newImplementation = func() (EmbeddingImpl, error) { return fixture, nil }
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		response *api.EmbeddingResponse
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		response, err := engine.GenerateEmbeddingsForModel(ctx, "model.gguf", DefaultEmbeddingOptions(), &api.EmbeddingRequest{Model: "model", Input: "text"})
		done <- outcome{response, err}
	}()
	<-fixture.embedStart
	cancel()
	// A cancelled call retains its admission until the non-cooperative backend
	// returns; another provider may not load on top of it.
	nativeCtx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if release, _, err := scheduler.acquireFamily(nativeCtx, true); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("native load overlapped cancelled embedding: release=%v err=%v", release != nil, err)
	}
	close(fixture.embedFinish)
	got := <-done
	if got.response != nil || !errors.Is(got.err, context.Canceled) {
		t.Fatalf("late vectors crossed Stop: response=%v err=%v", got.response, got.err)
	}
	if timing := scheduler.EmbeddingTimingSnapshot(); timing.Samples != 2 || timing.Generation <= 0 || timing.Cancel <= 0 {
		t.Fatalf("cancelled embedding timing was not recorded: %+v", timing)
	}
	release, _, err := scheduler.acquireFamily(context.Background(), true)
	if err != nil {
		t.Fatalf("completed cancellation leaked scheduler admission: %v", err)
	}
	release()
}

func (f *providerEmbeddingFixture) Load(ctx context.Context, _ string, _ EmbeddingOptions) error {
	return ctx.Err()
}
func (f *providerEmbeddingFixture) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if f.started != nil {
		close(f.started)
		select {
		case <-f.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	vectors := make([][]float32, len(texts))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	return vectors, nil
}
func (f *providerEmbeddingFixture) Unload() error {
	if f.unloads != nil {
		f.unloads.Add(1)
	}
	return nil
}
func (*providerEmbeddingFixture) GetDimensions() int { return 2 }
func (f *providerEmbeddingFixture) Identity() string { return f.identity }

func TestEmbeddingProviderRetirementReloadsOnlyMatchingKnowledgeIdentity(t *testing.T) {
	var loads, unloads atomic.Int32
	engine := NewEmbeddingEngine()
	engine.newImplementation = func() (EmbeddingImpl, error) {
		loads.Add(1)
		return &providerEmbeddingFixture{identity: "verified-digest", unloads: &unloads}, nil
	}
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	retired, err := engine.RetireForProvider()
	if err != nil || !retired || engine.IsLoaded() || unloads.Load() != 1 {
		t.Fatalf("embedding process was not retired: retired=%v err=%v unloads=%d", retired, err, unloads.Load())
	}
	if retired, err := engine.RetireForProvider(); err != nil || retired {
		t.Fatalf("parked embedding retired twice: retired=%v err=%v", retired, err)
	}
	req := &api.EmbeddingRequest{Model: "model", Input: "text"}
	if _, err := engine.GenerateEmbeddingsForIdentity(context.Background(), req, "different-digest"); err == nil || loads.Load() != 1 {
		t.Fatalf("wrong identity reloaded parked runtime: loads=%d err=%v", loads.Load(), err)
	}
	if _, err := engine.GenerateEmbeddingsForIdentity(context.Background(), req, "verified-digest"); err != nil || loads.Load() != 2 {
		t.Fatalf("matching identity did not reload safely: loads=%d err=%v", loads.Load(), err)
	}
	if err := engine.Unload(); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.GenerateEmbeddingsForIdentity(context.Background(), req, "verified-digest"); err == nil || loads.Load() != 2 {
		t.Fatalf("explicit unload silently reloaded runtime: loads=%d err=%v", loads.Load(), err)
	}
}

func TestEmbeddingProviderRetirementRejectsChangedRuntimeIdentity(t *testing.T) {
	var loads atomic.Int32
	engine := NewEmbeddingEngine()
	engine.newImplementation = func() (EmbeddingImpl, error) {
		if loads.Add(1) == 1 {
			return &providerEmbeddingFixture{identity: "old"}, nil
		}
		return &providerEmbeddingFixture{identity: "new"}, nil
	}
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.RetireForProvider(); err != nil {
		t.Fatal(err)
	}
	_, err := engine.GenerateEmbeddingsForIdentity(context.Background(), &api.EmbeddingRequest{Model: "model", Input: "text"}, "old")
	if err == nil || engine.IsLoaded() {
		t.Fatalf("changed embedding runtime was accepted: loaded=%v err=%v", engine.IsLoaded(), err)
	}
}

func TestEmbeddingLoadAndGenerationRespectProviderAdmission(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	engine := NewEmbeddingEngine()
	engine.SetProviderScheduler(scheduler)
	var fixture *providerEmbeddingFixture
	engine.newImplementation = func() (EmbeddingImpl, error) {
		fixture = &providerEmbeddingFixture{identity: "verified", started: make(chan struct{}), resume: make(chan struct{})}
		return fixture, nil
	}
	releaseNative, _, err := scheduler.acquireFamily(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	loadCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := engine.Load(loadCtx, "model.gguf", DefaultEmbeddingOptions()); !errors.Is(err, context.DeadlineExceeded) || fixture != nil {
		t.Fatalf("embedding load bypassed native admission: fixture=%v err=%v", fixture, err)
	}
	releaseNative()
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := engine.GenerateEmbeddingsForIdentity(context.Background(), &api.EmbeddingRequest{Model: "model", Input: "text"}, "verified")
		done <- err
	}()
	<-fixture.started
	nativeCtx, cancelNative := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancelNative()
	if release, _, err := scheduler.acquireFamily(nativeCtx, true); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("native admission overlapped embedding generation: lease=%v err=%v", release != nil, err)
	}
	close(fixture.resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	release, _, err := scheduler.acquireFamily(context.Background(), true)
	if err != nil {
		t.Fatalf("finished embedding leaked generation admission: %v", err)
	}
	release()
}

func TestNativeProviderSwitchRetiresEmbeddingAndKnowledgeCanResume(t *testing.T) {
	registry := NewProviderRegistry()
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	engine := NewEmbeddingEngine()
	engine.SetProviderScheduler(scheduler)
	var unloads atomic.Int32
	engine.newImplementation = func() (EmbeddingImpl, error) {
		return &providerEmbeddingFixture{identity: "verified", unloads: &unloads}, nil
	}
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) {
		return engine.RetireForProvider()
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Load(context.Background(), "model.gguf", DefaultEmbeddingOptions()); err != nil {
		t.Fatal(err)
	}
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	if err := registry.Register(ProviderTransformers, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if engine.IsLoaded() || unloads.Load() != 1 {
			return nil, errors.New("native worker opened while embedding process remained resident")
		}
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	req := &api.EmbeddingRequest{Model: "model", Input: "text"}
	if _, err := engine.GenerateEmbeddingsForIdentity(context.Background(), req, "verified"); err != nil || !engine.IsLoaded() {
		t.Fatalf("knowledge did not resume after native switch: loaded=%v err=%v", engine.IsLoaded(), err)
	}
}
