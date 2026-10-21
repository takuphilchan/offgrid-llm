package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderSchedulerReportsCacheOwnedLlamaResidencyWithoutClaimingMemory(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var residents atomic.Int32
	var observerUnlocked atomic.Bool
	residents.Store(2)
	if err := scheduler.ConfigureLegacyResidency(func() int {
		if scheduler.mu.TryLock() {
			observerUnlocked.Store(true)
			scheduler.mu.Unlock()
		}
		return int(residents.Load())
	}); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.ConfigureLegacyResidency(func() int { return 0 }); err == nil {
		t.Fatal("replaced the startup-only residency observer")
	}
	if got := scheduler.MemorySnapshot(); got.LegacyUnmeasuredResidents != 2 || len(got.Devices) != 0 {
		t.Fatalf("legacy processes were reported as measured/free memory: %+v", got)
	}
	if !observerUnlocked.Load() {
		t.Fatal("legacy residency was observed while holding the scheduler lock")
	}
	residents.Store(0)
	if got := scheduler.MemorySnapshot(); got.LegacyUnmeasuredResidents != 0 {
		t.Fatalf("retired legacy processes remain in snapshot: %+v", got)
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderSchedulerKeepsLegacyRSSSeparateFromDeviceBudget(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.legacyMemory = func() LegacyProcessMemory {
		return LegacyProcessMemory{Residents: 2, RSSBytes: 1024, RSSMeasured: 1, RSSUnmeasured: 1}
	}
	got := scheduler.MemorySnapshot()
	if got.LegacyUnmeasuredResidents != 2 || got.LegacyResidentRSSBytes != 1024 || got.LegacyRSSMeasured != 1 || got.LegacyRSSUnmeasured != 1 || len(got.Devices) != 0 {
		t.Fatalf("host RSS was confused with measured device capacity: %+v", got)
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
}

type memoryRuntime struct {
	schedulerFake
	report ProviderResources
}

type changingMemoryRuntime struct {
	memoryRuntime
	failOnce bool
}

func (r *changingMemoryRuntime) Generate(ctx context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	if r.failOnce {
		r.failOnce = false
		r.report.MemoryBytes = 800
		return errors.New("generation failed after memory growth")
	}
	return r.schedulerFake.Generate(ctx, request, callback)
}

func (r *memoryRuntime) Resources(context.Context) (ProviderResources, error) {
	return r.report, nil
}

func TestProviderSchedulerTracksResidentMemoryThroughSwitch(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cuda", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	workers := map[string]*memoryRuntime{}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		memory := int64(256)
		if descriptor.ModelDigest == second.ModelDigest {
			memory = 512
		}
		worker := &memoryRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, report: ProviderResources{Backend: "cuda", Device: "GPU 0", MemoryBytes: memory, MemoryLimitBytes: 1024, MemoryMeasured: true}}
		workers[descriptor.ModelDigest] = worker
		return worker, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if got := scheduler.MemorySnapshot(); len(got.Devices) != 1 || got.Devices[0].MeasuredBytes != 256 || got.Devices[0].ResidentCount != 1 {
		t.Fatalf("first resident memory was not accounted: %+v", got)
	}
	if _, err := scheduler.Runtime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if got := scheduler.MemorySnapshot(); len(got.Devices) != 1 || got.Devices[0].MeasuredBytes != 512 || got.Devices[0].ResidentCount != 1 || workers[first.ModelDigest].closes.Load() != 1 {
		t.Fatalf("switch retained stale memory or process ownership: %+v", got)
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	if got := scheduler.MemorySnapshot(); len(got.Devices) != 0 || got.Unresolved {
		t.Fatalf("clean shutdown retained memory claim: %+v", got)
	}
}

func TestProviderSchedulerInvalidatesMemoryClaimAfterFailedGeneration(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:memory-change", Format: FormatSafetensors, Backend: "cuda", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &changingMemoryRuntime{memoryRuntime: memoryRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, report: ProviderResources{Backend: "cuda", Device: "GPU 0", MemoryBytes: 256, MemoryLimitBytes: 1024, MemoryMeasured: true}}, failOnce: true}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	if timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil || timing.Resources.MemoryMeasured {
		t.Fatalf("failed generation retained its old memory claim: timing=%+v err=%v", timing, err)
	}
	if snapshot := scheduler.MemorySnapshot(); len(snapshot.Devices) != 1 || snapshot.Devices[0].MeasuredBytes != 0 || snapshot.Devices[0].UnmeasuredResidents != 1 {
		t.Fatalf("failed generation was reported as measured resident memory: %+v", snapshot)
	}
	request.RequestID, request.CancellationID = "second", "second"
	if _, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if snapshot := scheduler.MemorySnapshot(); len(snapshot.Devices) != 1 || snapshot.Devices[0].MeasuredBytes != 800 || snapshot.Devices[0].UnmeasuredResidents != 0 {
		t.Fatalf("next admission did not refresh resident memory: %+v", snapshot)
	}
}

func TestProviderSchedulerRejectsUnmeasuredOrOverLimitOverlap(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	scheduler.mu.Lock()
	first := ProviderResources{Backend: "cuda", Device: "GPU 0", MemoryBytes: 700, MemoryLimitBytes: 1024, MemoryMeasured: true}
	firstErr := scheduler.accountResidentMemoryLocked("first", first)
	second := ProviderResources{Backend: "cuda", Device: "GPU 0", MemoryBytes: 400, MemoryLimitBytes: 1024, MemoryMeasured: true}
	secondErr := scheduler.accountResidentMemoryLocked("second", second)
	unknownErr := scheduler.accountResidentMemoryLocked("second", ProviderResources{Backend: "cuda", Device: "GPU 0"})
	scheduler.mu.Unlock()
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if secondErr == nil {
		t.Fatal("over-limit overlap was admitted")
	}
	if unknownErr == nil {
		t.Fatal("unmeasured overlap was admitted")
	}
	if got := scheduler.MemorySnapshot(); len(got.Devices) != 1 || got.Devices[0].MeasuredBytes != 700 || got.Devices[0].ResidentCount != 1 {
		t.Fatalf("rejected overlap changed resident accounting: %+v", got)
	}
}

func TestProviderSchedulerDoesNotReportUnknownMemoryAsFree(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:fixture", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	got := scheduler.MemorySnapshot()
	if len(got.Devices) != 1 || got.Devices[0].ResidentCount != 1 || got.Devices[0].UnmeasuredResidents != 1 || got.Devices[0].MeasuredBytes != 0 {
		t.Fatalf("unmeasured resident was presented as known free capacity: %+v", got)
	}
}

func TestProviderSchedulerMemorySnapshotMarksUnpublishedLoadUnresolved(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:pending", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	started, release := make(chan struct{}), make(chan struct{})
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		close(started)
		<-release
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	loaded := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); loaded <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider load did not start")
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.PendingTransitions != 1 || !snapshot.Unresolved || len(snapshot.Devices) != 0 {
		t.Fatalf("unpublished worker was reported as free capacity: %+v", snapshot)
	}
	close(release)
	if err := <-loaded; err != nil {
		t.Fatal(err)
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.PendingTransitions != 0 || snapshot.Unresolved || len(snapshot.Devices) != 1 || snapshot.Devices[0].UnmeasuredResidents != 1 {
		t.Fatalf("settled worker has incorrect memory state: %+v", snapshot)
	}
}

func TestProviderSchedulerMemorySnapshotTracksNormalRetirement(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:retiring", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	closing, finishClose := make(chan struct{}, 1), make(chan struct{})
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		return &schedulerFake{descriptor: descriptor, closeStart: closing, closeBlock: finishClose}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	admitted := make(chan error, 1)
	go func() {
		release, _, err := scheduler.AcquireGenerationTimed(context.Background())
		if err == nil {
			release()
		}
		admitted <- err
	}()
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("native worker did not enter retirement")
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.PendingTransitions != 1 || !snapshot.Unresolved || len(snapshot.Devices) != 0 {
		t.Fatalf("retiring worker was reported as settled capacity: %+v", snapshot)
	}
	close(finishClose)
	if err := <-admitted; err != nil {
		t.Fatal(err)
	}
	if snapshot := scheduler.MemorySnapshot(); snapshot.PendingTransitions != 0 || snapshot.Unresolved || len(snapshot.Devices) != 0 {
		t.Fatalf("completed retirement remained unresolved: %+v", snapshot)
	}
}

func TestProviderSchedulerMemorySnapshotMarksRetiringWorkerUnresolved(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:fixture", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	closing := make(chan struct{}, 1)
	release := make(chan struct{})
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		return &schedulerFake{descriptor: descriptor, closeStart: closing, closeBlock: release}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- scheduler.Close() }()
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("provider did not begin retirement")
	}
	if got := scheduler.MemorySnapshot(); !got.Unresolved {
		t.Fatalf("retiring worker was reported as settled: %+v", got)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := scheduler.MemorySnapshot(); got.Unresolved || len(got.Devices) != 0 {
		t.Fatalf("clean retirement remained unresolved: %+v", got)
	}
}
