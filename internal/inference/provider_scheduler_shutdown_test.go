package inference

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderSchedulerCloseDrainsPendingWorkerStartup(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &schedulerFake{descriptor: descriptor}
	started := make(chan struct{})
	release := make(chan struct{})
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(ctx context.Context, _ ProviderDescriptor) (ProviderRuntime, error) {
		close(started)
		<-ctx.Done()
		<-release
		return worker, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	loaded := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); loaded <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker startup did not begin")
	}
	closed := make(chan error, 1)
	go func() { closed <- scheduler.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("scheduler returned before pending worker cleanup: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := <-loaded; err == nil || worker.closes.Load() != 1 {
		t.Fatalf("rejected startup was not retired: err=%v closes=%d", err, worker.closes.Load())
	}
	if err := scheduler.Close(); err != nil {
		t.Fatalf("repeat close changed settled outcome: %v", err)
	}
}

func TestProviderSchedulerCloseDrainsGeneration(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &slowCancellationRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "shutdown", CancellationID: "shutdown", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	generated := make(chan error, 1)
	go func() {
		generated <- scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("generation did not begin")
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-generated; !errors.Is(err, context.Canceled) || worker.closes.Load() != 1 {
		t.Fatalf("generation remained active after clean close: err=%v closes=%d", err, worker.closes.Load())
	}
	if worker.closedEarly.Load() {
		t.Fatal("provider worker was retired while its cancelled generation was still active")
	}
}

func TestProviderSchedulerCloseDrainsInFlightStopSignal(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &delayedCancelSchedulerRuntime{blockingSchedulerRuntime: blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 1), release: make(chan struct{})}, entered: make(chan struct{}, 1), resume: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "shutdown-stop", CancellationID: "shutdown-stop", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	generated := make(chan error, 1)
	go func() {
		generated <- scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("generation did not begin")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- scheduler.Cancel(context.Background(), descriptor, request.CancellationID) }()
	select {
	case <-worker.entered:
	case <-time.After(time.Second):
		t.Fatal("request-scoped cancellation did not begin")
	}
	if err := <-generated; !errors.Is(err, context.Canceled) {
		t.Fatalf("generation did not drain after Stop: %v", err)
	}
	closed := make(chan error, 1)
	go func() { closed <- scheduler.Close() }()
	select {
	case err := <-closed:
		close(worker.resume)
		t.Fatalf("scheduler retired a worker while Stop was still in flight: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if worker.closes.Load() != 0 {
		close(worker.resume)
		t.Fatal("worker was closed before its Stop signal returned")
	}
	close(worker.resume)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil || worker.closes.Load() != 1 {
		t.Fatalf("worker was not retired after Stop settled: err=%v closes=%d", err, worker.closes.Load())
	}
}

func TestProviderSchedulerCloseWaitsForLegacyLease(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	release, _, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- scheduler.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("scheduler returned with an active legacy model lease: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	release()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestProviderSchedulerOwnsLegacyShutdownAfterLeaseDrain(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var shutdowns atomic.Int32
	if err := scheduler.ConfigureLegacyShutdown(func() error { shutdowns.Add(1); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.ConfigureLegacyShutdown(func() error { return nil }); err == nil {
		t.Fatal("legacy shutdown owner was replaced")
	}
	release, _, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- scheduler.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("closed before legacy lease drained: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if shutdowns.Load() != 0 {
		t.Fatal("legacy cache retired under active model lease")
	}
	release()
	if err := <-closed; err != nil || shutdowns.Load() != 1 {
		t.Fatalf("legacy shutdown did not follow lease drain: err=%v calls=%d", err, shutdowns.Load())
	}
	if err := scheduler.Close(); err != nil || shutdowns.Load() != 1 {
		t.Fatalf("legacy shutdown ran more than once: err=%v calls=%d", err, shutdowns.Load())
	}
}

func TestProviderSchedulerReportsLegacyShutdownFailure(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("legacy process remains")
	if err := scheduler.ConfigureLegacyShutdown(func() error { return failure }); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Close(); !errors.Is(err, failure) {
		t.Fatalf("legacy process failure was hidden: %v", err)
	}
	if err := scheduler.Close(); !errors.Is(err, failure) {
		t.Fatalf("second close lost the failed shutdown outcome: %v", err)
	}
}

func TestProviderSchedulerAttachesLlamaCacheAsOneOwner(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewModelCache(1, 0, t.TempDir())
	process, err := scheduler.AttachLlamaCache(cache, NewMockEngine())
	if err != nil || process == nil {
		cache.StopMonitor()
		t.Fatalf("attach llama process: %v", err)
	}
	if cache.restartAdmission == nil || scheduler.legacyResident == nil || scheduler.legacyShutdown == nil {
		t.Fatal("llama ownership was only partially attached")
	}
	if _, err := scheduler.AttachLlamaCache(cache, NewMockEngine()); err == nil {
		t.Fatal("second llama owner replaced the first")
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cache.monitorDone:
	default:
		t.Fatal("scheduler shutdown did not join the llama monitor")
	}
	if release, err := cache.restartAdmission(context.Background()); err == nil {
		if release != nil {
			release()
		}
		t.Fatal("closed scheduler admitted a llama restart")
	}
}

func TestAttachedLlamaProcessRequiresSchedulerLifecycleLease(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	cache := NewModelCache(1, 0, t.TempDir())
	process, err := scheduler.AttachLlamaCache(cache, NewMockEngine())
	if err != nil {
		cache.StopMonitor()
		t.Fatal(err)
	}
	if _, err := process.Activate(context.Background(), "model", "model.gguf", "", LoadOptions{ContextSize: 4096}); err == nil || !strings.Contains(err.Error(), "scheduler admission") {
		t.Fatalf("unadmitted activation reached the cache: %v", err)
	}
	if retired, err := process.RetireAll(context.Background()); retired || err == nil || !strings.Contains(err.Error(), "scheduler admission") {
		t.Fatalf("unadmitted retirement reached the cache: retired=%v err=%v", retired, err)
	}
	gate := NewLifecycleGate(1)
	called := false
	var expiredContext context.Context
	if release, _, err := scheduler.AcquireLegacyModel(context.Background(), gate, "model", 4096, nil, func(ctx context.Context, model string) error {
		called = true
		expiredContext = ctx
		_, activationErr := process.Activate(ctx, model, "", "", LoadOptions{ContextSize: 4096})
		if activationErr == nil || !strings.Contains(activationErr.Error(), "model identity") {
			t.Fatalf("scheduler-admitted activation did not reach argument validation: %v", activationErr)
		}
		return nil
	}); err != nil || !called {
		t.Fatalf("scheduler did not authorize its own activation callback: called=%v err=%v", called, err)
	} else {
		release()
	}
	if _, err := process.Activate(expiredContext, "model", "model.gguf", "", LoadOptions{ContextSize: 4096}); err == nil || !strings.Contains(err.Error(), "scheduler admission") {
		t.Fatalf("completed lifecycle callback retained activation authority: %v", err)
	}
	if retired, err := process.RetireAll(expiredContext); retired || err == nil || !strings.Contains(err.Error(), "scheduler admission") {
		t.Fatalf("completed lifecycle callback retained retirement authority: retired=%v err=%v", retired, err)
	}
	if err := scheduler.ConfigureLegacyRetirement(func(ctx context.Context) (bool, error) { return process.RetireAll(ctx) }); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.retireLegacyBeforeNative(context.Background()); err != nil {
		t.Fatalf("scheduler did not authorize native-switch retirement: %v", err)
	}
}

func TestProviderSchedulerRejectsPartialLlamaOwnership(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewModelCache(1, 0, t.TempDir())
	defer cache.StopMonitor()
	defer scheduler.Close()
	if err := scheduler.ConfigureLegacyResidency(func() int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.AttachLlamaCache(cache, NewMockEngine()); err == nil {
		t.Fatal("attached llama cache over an existing partial owner")
	}
	if cache.restartAdmission != nil || scheduler.legacyShutdown != nil {
		t.Fatal("rejected llama attachment mutated process ownership")
	}
}

func TestProviderSchedulerRejectsActiveLlamaOwnershipTransfer(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	cache := NewModelCache(1, 0, t.TempDir())
	defer cache.StopMonitor()
	engine := &llamaProcessEngineFixture{loaded: true, actions: new([]string)}
	if _, err := scheduler.AttachLlamaCache(cache, engine); err == nil {
		t.Fatal("adopted an engine with unknown in-flight work")
	}
	if cache.restartAdmission != nil || scheduler.legacyShutdown != nil || scheduler.legacyResident != nil {
		t.Fatal("rejected active owner changed scheduler state")
	}
}
