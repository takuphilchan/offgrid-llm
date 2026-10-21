package inference

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type schedulerFake struct {
	closes     atomic.Int32
	generates  atomic.Int32
	descriptor ProviderDescriptor
	closeErr   error
	closeDelay time.Duration
	closeStart chan<- struct{}
	closeBlock <-chan struct{}
}

type healthHandshakeRuntime struct {
	schedulerFake
	entered chan struct{}
	release <-chan struct{}
}

type failingHealthRuntime struct{ schedulerFake }

type failingResidentHealthRuntime struct {
	schedulerFake
	checks atomic.Int32
}

func (r *failingResidentHealthRuntime) Health(context.Context) error {
	if r.checks.Add(1) == 1 {
		return nil
	}
	return errors.New("resident health failed")
}

type residentHealthGateRuntime struct {
	schedulerFake
	checks  atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (r *residentHealthGateRuntime) Health(ctx context.Context) error {
	if r.checks.Add(1) == 1 {
		return nil
	}
	close(r.entered)
	<-r.release
	return ctx.Err()
}

func (*failingHealthRuntime) Health(context.Context) error {
	return errors.New("fixture health failure")
}

func (r *healthHandshakeRuntime) Health(ctx context.Context) error {
	select {
	case r.entered <- struct{}{}:
	default:
	}
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestProviderSchedulerPinsResidentDuringHealthBeforeModelSwitch(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	closed := make(chan struct{}, 1)
	firstRuntime := &residentHealthGateRuntime{schedulerFake: schedulerFake{descriptor: first, closeStart: closed}, entered: make(chan struct{}), release: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		if descriptor.ModelDigest == first.ModelDigest {
			return firstRuntime, nil
		}
		return &schedulerFake{descriptor: second}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	healthDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), first); healthDone <- err }()
	select {
	case <-firstRuntime.entered:
	case <-time.After(time.Second):
		t.Fatal("resident health check did not start")
	}
	switchDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), second); switchDone <- err }()
	select {
	case <-closed:
		t.Fatal("model switch closed a worker during its health check")
	case <-time.After(30 * time.Millisecond):
	}
	close(firstRuntime.release)
	if err := <-healthDone; err != nil {
		t.Fatal(err)
	}
	if err := <-switchDone; err != nil || firstRuntime.closes.Load() != 1 {
		t.Fatalf("model switch did not retire idle worker after health: err=%v closes=%d", err, firstRuntime.closes.Load())
	}
}

func TestProviderSchedulerCloseDrainsResidentHealthCheck(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	closed := make(chan struct{}, 1)
	worker := &residentHealthGateRuntime{schedulerFake: schedulerFake{descriptor: descriptor, closeStart: closed}, entered: make(chan struct{}), release: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	healthDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); healthDone <- err }()
	select {
	case <-worker.entered:
	case <-time.After(time.Second):
		t.Fatal("resident health check did not start")
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- scheduler.Close() }()
	select {
	case <-closed:
		t.Fatal("shutdown closed a worker during its health check")
	case <-time.After(30 * time.Millisecond):
	}
	close(worker.release)
	if err := <-healthDone; err == nil {
		t.Fatal("health check succeeded after scheduler shutdown")
	}
	if err := <-shutdown; err != nil || worker.closes.Load() != 1 {
		t.Fatalf("shutdown did not retire worker after health check: err=%v closes=%d", err, worker.closes.Load())
	}
}

func TestProviderSchedulerCloseDrainsFailedResidentRetirement(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	closeStarted := make(chan struct{}, 1)
	releaseClose := make(chan struct{})
	retirementFailure := errors.New("worker termination uncertain")
	worker := &failingResidentHealthRuntime{schedulerFake: schedulerFake{descriptor: descriptor, closeErr: retirementFailure, closeStart: closeStarted, closeBlock: releaseClose}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	healthDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); healthDone <- err }()
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("failed resident was not being retired")
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- scheduler.Close() }()
	select {
	case err := <-shutdown:
		t.Fatalf("shutdown returned before uncertain retirement settled: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseClose)
	if err := <-healthDone; !errors.Is(err, retirementFailure) {
		t.Fatalf("health path lost retirement failure: %v", err)
	}
	if err := <-shutdown; !errors.Is(err, retirementFailure) || worker.closes.Load() != 1 {
		t.Fatalf("shutdown lost uncertain retirement: err=%v closes=%d", err, worker.closes.Load())
	}
}

func TestProviderSchedulerSwitchWaitsForFailedResidentCleanup(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	closeStarted := make(chan struct{}, 1)
	releaseClose := make(chan struct{})
	worker := &failingResidentHealthRuntime{schedulerFake: schedulerFake{descriptor: first, closeStart: closeStarted, closeBlock: releaseClose}}
	var secondOpened atomic.Int32
	var firstOpened atomic.Int32
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		if descriptor.ModelDigest == first.ModelDigest {
			if firstOpened.Add(1) == 1 {
				return worker, nil
			}
			return &schedulerFake{descriptor: first}, nil
		}
		secondOpened.Add(1)
		return &schedulerFake{descriptor: second}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	retirement := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), first); retirement <- err }()
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("failed resident cleanup did not start")
	}
	switchDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), second); switchDone <- err }()
	select {
	case err := <-switchDone:
		t.Fatalf("switch passed in-progress cleanup: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if secondOpened.Load() != 0 {
		t.Fatal("second provider opened before prior cleanup")
	}
	close(releaseClose)
	if err := <-retirement; err != nil {
		t.Fatal(err)
	}
	if err := <-switchDone; err != nil || secondOpened.Load() != 1 {
		t.Fatalf("switch did not resume after cleanup: err=%v opens=%d", err, secondOpened.Load())
	}
}

func TestProviderSchedulerSerializesLoadUntilHandshakeIsResident(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:first", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	entered := make(chan struct{}, 1)
	releaseHealth := make(chan struct{})
	secondOpened := make(chan struct{}, 1)
	firstRuntime := &healthHandshakeRuntime{schedulerFake: schedulerFake{descriptor: first}, entered: entered, release: releaseHealth}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		if descriptor.ModelDigest == first.ModelDigest {
			return firstRuntime, nil
		}
		secondOpened <- struct{}{}
		return &schedulerFake{descriptor: second}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	firstDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), first); firstDone <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first provider did not reach its health handshake")
	}
	secondDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), second); secondDone <- err }()
	select {
	case <-secondOpened:
		t.Fatal("second model loaded before first handshake became resident")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseHealth)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if firstRuntime.closes.Load() != 1 {
		t.Fatalf("first resident was not retired before second load: closes=%d", firstRuntime.closes.Load())
	}
}

func TestProviderSchedulerClosesRejectedWorkerBeforeAnotherLoad(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:first", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	closeStarted := make(chan struct{}, 1)
	releaseClose := make(chan struct{})
	secondOpened := make(chan struct{}, 1)
	firstRuntime := &failingHealthRuntime{schedulerFake{descriptor: first, closeStart: closeStarted, closeBlock: releaseClose}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		if descriptor.ModelDigest == first.ModelDigest {
			return firstRuntime, nil
		}
		secondOpened <- struct{}{}
		return &schedulerFake{descriptor: second}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	firstDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), first); firstDone <- err }()
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("rejected worker was not closed")
	}
	secondDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), second); secondDone <- err }()
	select {
	case <-secondOpened:
		t.Fatal("second model loaded before rejected worker exited")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseClose)
	if err := <-firstDone; err == nil {
		t.Fatal("unhealthy worker was accepted")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}

func TestProviderSchedulerCloseCancelsHealthHandshake(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:first", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	entered := make(chan struct{}, 1)
	worker := &healthHandshakeRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, entered: entered, release: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach health handshake")
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || worker.closes.Load() != 1 {
			t.Fatalf("close left an unverified worker active: err=%v closes=%d", err, worker.closes.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("close did not cancel the provider health handshake")
	}
}

func TestProviderSchedulerLoadTimingIncludesHealthHandshake(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:first", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	entered := make(chan struct{}, 1)
	releaseHealth := make(chan struct{})
	worker := &healthHandshakeRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, entered: entered, release: releaseHealth}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	type outcome struct {
		timing ProviderTiming
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		timing, err := scheduler.GenerateTimed(context.Background(), descriptor, ProviderRequest{RequestID: "timed", CancellationID: "timed", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}, func(ProviderEvent) error { return nil })
		done <- outcome{timing, err}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not enter health handshake")
	}
	time.Sleep(25 * time.Millisecond)
	close(releaseHealth)
	result := <-done
	if result.err != nil || result.timing.Load < 20*time.Millisecond {
		t.Fatalf("load timing excluded provider readiness: timing=%+v err=%v", result.timing, result.err)
	}
}

func waitForScheduledRequest(t *testing.T, scheduler *ProviderScheduler, descriptor ProviderDescriptor, id string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scheduler.mu.Lock()
		registered := scheduler.requests[scheduledKey(descriptor, id)] != nil
		scheduler.mu.Unlock()
		if registered {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("provider request was not registered before admission")
}

func TestProviderSchedulerCancelsQueuedAndLoadingRequests(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	startedLoad := make(chan struct{}, 1)
	var loads atomic.Int32
	if err := registry.Register(ProviderTransformers, func(ctx context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		loads.Add(1)
		startedLoad <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	queued := ProviderRequest{RequestID: "queued", CancellationID: "cancel-queued", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	queuedDone := make(chan error, 1)
	go func() {
		queuedDone <- scheduler.Generate(context.Background(), descriptor, queued, func(ProviderEvent) error { return nil })
	}()
	waitForScheduledRequest(t, scheduler, descriptor, queued.CancellationID)
	if err := scheduler.Cancel(context.Background(), descriptor, queued.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-queuedDone; !errors.Is(err, context.Canceled) || loads.Load() != 0 {
		t.Fatalf("queued Stop opened a worker or did not cancel: loads=%d err=%v", loads.Load(), err)
	}
	held()
	loading := ProviderRequest{RequestID: "loading", CancellationID: "cancel-loading", ModelDigest: descriptor.ModelDigest, Prompt: queued.Prompt}
	loadingDone := make(chan error, 1)
	go func() {
		loadingDone <- scheduler.Generate(context.Background(), descriptor, loading, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-startedLoad:
	case <-time.After(time.Second):
		t.Fatal("worker load did not begin")
	}
	if err := scheduler.Cancel(context.Background(), descriptor, loading.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-loadingDone; !errors.Is(err, context.Canceled) || loads.Load() != 1 {
		t.Fatalf("loading Stop failed: loads=%d err=%v", loads.Load(), err)
	}
}

func TestProviderSchedulerRejectsConcurrentRequestIdentities(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	scheduler, _ := NewProviderScheduler(NewProviderRegistry(), 1)
	defer scheduler.Close()
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	request := ProviderRequest{RequestID: "request", CancellationID: "cancel-request", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	type outcome struct {
		timing ProviderTiming
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
		done <- outcome{timing, err}
	}()
	waitForScheduledRequest(t, scheduler, descriptor, request.CancellationID)
	duplicateCancel := request
	duplicateCancel.RequestID = "another-request"
	if err := scheduler.Generate(context.Background(), descriptor, duplicateCancel, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("duplicate active cancellation identity was accepted")
	}
	duplicateRequest := request
	duplicateRequest.CancellationID = "another-cancellation"
	if err := scheduler.Generate(context.Background(), descriptor, duplicateRequest, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("duplicate active request identity was accepted")
	}
	if err := scheduler.Cancel(context.Background(), descriptor, request.CancellationID); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("queued Stop did not cancel the original request: %+v, %v", result.timing, result.err)
	}
}

func TestProviderSchedulerRejectsInvalidDescriptorBeforeQueue(t *testing.T) {
	scheduler, _ := NewProviderScheduler(NewProviderRegistry(), 1)
	defer scheduler.Close()
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	request := ProviderRequest{RequestID: "invalid", CancellationID: "invalid", ModelDigest: "sha256:model", Prompt: []byte(`{"messages":[]}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := scheduler.GenerateTimed(ctx, ProviderDescriptor{Provider: ProviderMLX}, request, func(ProviderEvent) error { return nil }); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("invalid provider waited behind active inference: %v", err)
	}
}

type slowCancellationRuntime struct {
	schedulerFake
	started     chan struct{}
	finished    atomic.Bool
	closedEarly atomic.Bool
}

func (f *slowCancellationRuntime) Generate(ctx context.Context, _ ProviderRequest, _ func(ProviderEvent) error) error {
	defer f.finished.Store(true)
	close(f.started)
	<-ctx.Done()
	time.Sleep(15 * time.Millisecond)
	return ctx.Err()
}

func (f *slowCancellationRuntime) Close() error {
	if !f.finished.Load() {
		f.closedEarly.Store(true)
	}
	return f.schedulerFake.Close()
}

func TestProviderSchedulerMeasuresDelayedStop(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &slowCancellationRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "slow", CancellationID: "cancel-slow", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	type outcome struct {
		timing ProviderTiming
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
		done <- outcome{timing, err}
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not begin generation")
	}
	if err := scheduler.Cancel(context.Background(), descriptor, request.CancellationID); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if !errors.Is(result.err, context.Canceled) || result.timing.Cancel < 10*time.Millisecond {
		t.Fatalf("delayed Stop time was not measured: %+v, %v", result.timing, result.err)
	}
}

func TestProviderSchedulerCancelTargetsOneActiveRequest(t *testing.T) {
	// Existing llama runtimes may use explicitly configured parallel slots;
	// unqualified native workers are serialized even when those slots exist.
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:model", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &cancellableSchedulerRuntime{blockingSchedulerRuntime: blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 2), release: make(chan struct{})}, cancelled: make(chan string, 1)}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	first := ProviderRequest{RequestID: "first", CancellationID: "cancel-first", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	second := ProviderRequest{RequestID: "second", CancellationID: "cancel-second", ModelDigest: descriptor.ModelDigest, Prompt: first.Prompt}
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		firstDone <- scheduler.Generate(context.Background(), descriptor, first, func(ProviderEvent) error { return nil })
	}()
	<-worker.started
	go func() {
		secondDone <- scheduler.Generate(context.Background(), descriptor, second, func(ProviderEvent) error { return nil })
	}()
	<-worker.started
	if err := scheduler.Cancel(context.Background(), descriptor, first.CancellationID); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-worker.cancelled:
		if id != first.CancellationID {
			t.Fatalf("provider cancelled %q instead of the selected request", id)
		}
	case <-time.After(time.Second):
		t.Fatal("active provider did not receive request-scoped cancellation")
	}
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("selected active request was not cancelled: %v", err)
	}
	select {
	case err := <-secondDone:
		t.Fatalf("unrelated request was cancelled: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(worker.release)
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}

func TestProviderSchedulerSendsCancelWhenStopCallerIsAlreadyCancelled(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:model", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &cancellableSchedulerRuntime{blockingSchedulerRuntime: blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 1), release: make(chan struct{})}, cancelled: make(chan string, 1)}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "request", CancellationID: "stop-request", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	finished := make(chan error, 1)
	go func() {
		finished <- scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not begin generation")
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := scheduler.Cancel(stopCtx, descriptor, request.CancellationID); err != nil {
		t.Fatalf("cancelled Stop transport prevented worker cancellation: %v", err)
	}
	select {
	case id := <-worker.cancelled:
		if id != request.CancellationID {
			t.Fatalf("cancelled wrong request %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not receive request-scoped cancellation")
	}
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("generation did not stop: %v", err)
	}
}

type delayedCancelSchedulerRuntime struct {
	blockingSchedulerRuntime
	entered chan struct{}
	resume  chan struct{}
}

func (f *delayedCancelSchedulerRuntime) Cancel(ctx context.Context, _ string) error {
	f.entered <- struct{}{}
	select {
	case <-f.resume:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestProviderSchedulerReservesCancellationIdentityUntilSignalFinishes(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, RunnerRevision: "r1", ModelDigest: "sha256:model", Format: FormatGGUF, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &delayedCancelSchedulerRuntime{blockingSchedulerRuntime: blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 1), release: make(chan struct{})}, entered: make(chan struct{}, 1), resume: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	first := ProviderRequest{RequestID: "first", CancellationID: "shared", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- scheduler.Generate(context.Background(), descriptor, first, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("first generation did not start")
	}
	stopDone := make(chan error, 1)
	go func() { stopDone <- scheduler.Cancel(context.Background(), descriptor, first.CancellationID) }()
	select {
	case <-worker.entered:
	case <-time.After(time.Second):
		t.Fatal("worker cancellation did not begin")
	}
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first generation did not stop: %v", err)
	}
	second := first
	second.RequestID = "second"
	if err := scheduler.Generate(context.Background(), descriptor, second, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("reused cancellation identity while old Stop signal was in flight")
	}
	close(worker.resume)
	if err := <-stopDone; err != nil {
		t.Fatal(err)
	}
	close(worker.release)
	if err := scheduler.Generate(context.Background(), descriptor, second, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatalf("identity remained reserved after Stop completed: %v", err)
	}
}

type measuredSchedulerRuntime struct {
	schedulerFake
	generated atomic.Bool
}

type delayedCloseSchedulerRuntime struct {
	schedulerFake
	delay time.Duration
}

type preDispatchSchedulerRuntime struct {
	schedulerFake
	resourceCalls atomic.Int32
	preDispatch   chan struct{}
	release       chan struct{}
}

func (f *preDispatchSchedulerRuntime) Resources(ctx context.Context) (ProviderResources, error) {
	if f.resourceCalls.Add(1) == 2 {
		close(f.preDispatch)
		select {
		case <-f.release:
		case <-ctx.Done():
			return ProviderResources{}, ctx.Err()
		}
	}
	return f.schedulerFake.Resources(ctx)
}

func TestProviderSchedulerPinsLoadedRuntimeBeforeDispatch(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	worker := &preDispatchSchedulerRuntime{schedulerFake: schedulerFake{descriptor: first}, preDispatch: make(chan struct{}), release: make(chan struct{})}
	registry := NewProviderRegistry()
	var secondOpens atomic.Int32
	if err := registry.Register(ProviderMLX, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if selected.ModelDigest == first.ModelDigest {
			return worker, nil
		}
		secondOpens.Add(1)
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- scheduler.Generate(context.Background(), first, ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: first.ModelDigest, Prompt: []byte(`{"messages":[]}`)}, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.preDispatch:
	case <-time.After(time.Second):
		t.Fatal("first request did not reach the pre-dispatch boundary")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := scheduler.Runtime(waitCtx, second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("another model displaced a pre-dispatch runtime: %v", err)
	}
	if worker.closes.Load() != 0 || secondOpens.Load() != 0 {
		t.Fatal("pre-dispatch worker was retired before its request finished")
	}
	close(worker.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), second); err != nil {
		t.Fatalf("safe switch after release failed: %v", err)
	}
	if worker.closes.Load() != 1 || secondOpens.Load() != 1 {
		t.Fatal("released runtime was not retired exactly once")
	}
}

func (f *delayedCloseSchedulerRuntime) Close() error {
	time.Sleep(f.delay)
	return f.schedulerFake.Close()
}

func TestProviderSchedulerSeparatesUnloadAndLoadTiming(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	prior := &delayedCloseSchedulerRuntime{schedulerFake: schedulerFake{descriptor: first}, delay: 15 * time.Millisecond}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if selected.ModelDigest == first.ModelDigest {
			return prior, nil
		}
		time.Sleep(15 * time.Millisecond)
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "second", CancellationID: "second", ModelDigest: second.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	timing, err := scheduler.GenerateTimed(context.Background(), second, request, func(ProviderEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if timing.Unload < 10*time.Millisecond || timing.Load < 10*time.Millisecond {
		t.Fatalf("unload and load were conflated: %+v", timing)
	}
}

func (f *measuredSchedulerRuntime) Resources(context.Context) (ProviderResources, error) {
	bytes := int64(1024)
	if f.generated.Load() {
		bytes = 2048
	}
	return ProviderResources{Backend: f.descriptor.Backend, Device: "fixture", MemoryBytes: bytes, MemoryMeasured: true}, nil
}

func (f *measuredSchedulerRuntime) Generate(_ context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	f.generated.Store(true)
	return callback(ProviderEvent{ProtocolRevision: f.descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: request.RequestID, Sequence: 1, FinishReason: "stop"})
}

func TestProviderSchedulerReportsCompletionResources(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &measuredSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "measured", CancellationID: "measured", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	if err != nil || timing.Resources.MemoryBytes != 2048 {
		t.Fatalf("completion memory was not reported: %+v, %v", timing.Resources, err)
	}
}

type unhealthySchedulerRuntime struct {
	schedulerFake
	unhealthy atomic.Bool
}

func (f *unhealthySchedulerRuntime) Health(context.Context) error {
	if f.unhealthy.Load() {
		return errors.New("worker exited")
	}
	return nil
}

func TestProviderSchedulerReplacesDeadResidentBeforeNewDispatch(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	dead := &unhealthySchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}}
	good := &schedulerFake{descriptor: descriptor}
	registry := NewProviderRegistry()
	var opens atomic.Int32
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		if opens.Add(1) == 1 {
			return dead, nil
		}
		return good, nil
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
	dead.unhealthy.Store(true)
	// The next request is new work, not a replay of an uncertain generation.
	request := ProviderRequest{RequestID: "new-request", CancellationID: "new-request", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if timing.Resources.Backend != "cpu" || timing.Resources.Device != "fixture" {
		t.Fatalf("missing selected runtime resource report: %+v", timing.Resources)
	}
	if opens.Load() != 2 || dead.generates.Load() != 0 || dead.closes.Load() != 1 || good.generates.Load() != 1 {
		t.Fatalf("incorrect bounded recovery: opens=%d dead=%d closed=%d good=%d", opens.Load(), dead.generates.Load(), dead.closes.Load(), good.generates.Load())
	}
}

func (f *schedulerFake) Descriptor(context.Context) (ProviderDescriptor, error) {
	return f.descriptor, nil
}
func (f *schedulerFake) Resources(context.Context) (ProviderResources, error) {
	return ProviderResources{Backend: f.descriptor.Backend, Device: "fixture", MemoryMeasured: false}, nil
}
func (f *schedulerFake) Generate(_ context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	f.generates.Add(1)
	return callback(ProviderEvent{ProtocolRevision: f.descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: request.RequestID, Sequence: 1, FinishReason: "stop"})
}
func (f *schedulerFake) Cancel(context.Context, string) error { return nil }
func (f *schedulerFake) Health(context.Context) error         { return nil }
func (f *schedulerFake) Close() error {
	if f.closeStart != nil {
		f.closeStart <- struct{}{}
	}
	if f.closeBlock != nil {
		<-f.closeBlock
	}
	if f.closeDelay > 0 {
		time.Sleep(f.closeDelay)
	}
	f.closes.Add(1)
	return f.closeErr
}

func TestProviderSchedulerCancelledDuringNativeRetirementDoesNotAdmitLegacy(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	closeStarted := make(chan struct{}, 1)
	allowClose := make(chan struct{})
	fake := &schedulerFake{descriptor: descriptor, closeStart: closeStarted, closeBlock: allowClose}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	defer func() {
		select {
		case <-allowClose:
		default:
			close(allowClose)
		}
	}()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	type admission struct {
		release func()
		err     error
	}
	done := make(chan admission, 1)
	go func() {
		release, _, err := scheduler.AcquireGenerationTimed(ctx)
		done <- admission{release, err}
	}()
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("native worker was not retired before legacy admission")
	}
	cancel()
	close(allowClose)
	result := <-done
	if !errors.Is(result.err, context.Canceled) || result.release != nil {
		t.Fatalf("cancelled legacy request received an admission lease: %+v", result)
	}
	// Cancellation must also release both family and numeric slots.
	release, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestProviderSchedulerRetiresNativeResidentBeforeLegacyLoad(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	fake := &schedulerFake{descriptor: descriptor, closeDelay: 5 * time.Millisecond}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	release, timing, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fake.closes.Load() != 1 {
		t.Fatal("legacy model load admitted while native worker remained resident")
	}
	if timing.Unload <= 0 {
		t.Fatalf("native retirement was not attributed to unload timing: %+v", timing)
	}
	release()
}

func TestProviderSchedulerLegacyAdmissionKeepsLlamaResident(t *testing.T) {
	descriptor := llamaProviderDescriptor(CapabilityText)
	fake := &schedulerFake{descriptor: descriptor}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	release, timing, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fake.closes.Load() != 0 {
		t.Fatal("legacy admission unnecessarily retired a compatible llama runtime")
	}
	if timing.Unload != 0 {
		t.Fatalf("compatible llama admission reported a provider unload: %+v", timing)
	}
	release()
}

func TestProviderSchedulerLlamaProviderSwitchRetiresNativeFirst(t *testing.T) {
	native := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:native", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	llama := llamaProviderDescriptor(CapabilityText)
	prior := &schedulerFake{descriptor: native, closeDelay: 5 * time.Millisecond}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return prior, nil }); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(ProviderLlamaCPP, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		if prior.closes.Load() != 1 {
			return nil, errors.New("llama opened before native retirement")
		}
		return &schedulerFake{descriptor: llama}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), native); err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "llama", CancellationID: "llama", ModelDigest: llama.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	timing, err := scheduler.GenerateTimed(context.Background(), llama, request, func(ProviderEvent) error { return nil })
	if err != nil || timing.Unload <= 0 {
		t.Fatalf("llama provider switch did not retire native first: timing=%+v err=%v", timing, err)
	}
}

func TestProviderSchedulerNativeLoadRetiresLegacyFirst(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:native", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var retired atomic.Bool
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		if !retired.Load() {
			return nil, errors.New("native worker opened before llama retirement")
		}
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) {
		time.Sleep(5 * time.Millisecond)
		retired.Store(true)
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) { return false, nil }); err == nil {
		t.Fatal("late replacement of process owner was allowed")
	}
	request := ProviderRequest{RequestID: "native", CancellationID: "native", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	if err != nil || timing.Unload <= 0 {
		t.Fatalf("native load bypassed llama retirement: timing=%+v err=%v", timing, err)
	}
}

func TestProviderSchedulerDirectRuntimeLoadUsesFamilyAndRetirementGate(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:native", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var retired atomic.Bool
	var opens atomic.Int32
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		opens.Add(1)
		if !retired.Load() {
			return nil, errors.New("direct runtime load bypassed llama retirement")
		}
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) {
		retired.Store(true)
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	legacyLease, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := scheduler.Runtime(waitCtx, descriptor); !errors.Is(err, context.DeadlineExceeded) || retired.Load() || opens.Load() != 0 {
		t.Fatalf("direct runtime load crossed active llama generation: retired=%v opens=%d err=%v", retired.Load(), opens.Load(), err)
	}
	legacyLease()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil || opens.Load() != 1 {
		t.Fatalf("direct runtime load did not retire llama first: opens=%d err=%v", opens.Load(), err)
	}
}

func TestProviderSchedulerLegacyRetirementFailureBlocksNativeRetry(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:native", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var opens atomic.Int32
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		opens.Add(1)
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	var retireCalls atomic.Int32
	if err := scheduler.ConfigureLegacyRetirement(func(context.Context) (bool, error) {
		retireCalls.Add(1)
		return true, errors.New("llama process status uncertain")
	}); err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "native", CancellationID: "native", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil || opens.Load() != 0 {
		t.Fatalf("native opened after uncertain llama retirement: opens=%d err=%v", opens.Load(), err)
	}
	request.RequestID, request.CancellationID = "retry", "retry"
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil || opens.Load() != 0 || retireCalls.Load() != 1 {
		t.Fatalf("native retried after unresolved retirement: opens=%d retire=%d err=%v", opens.Load(), retireCalls.Load(), err)
	}
}

func TestProviderSchedulerPreEffectRetirementCancellationAllowsNewRequest(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:native", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var opens atomic.Int32
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		opens.Add(1)
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	ctx, cancel := context.WithCancel(context.Background())
	var retireCalls atomic.Int32
	if err := scheduler.ConfigureLegacyRetirement(func(callCtx context.Context) (bool, error) {
		if retireCalls.Add(1) == 1 {
			cancel()
			return false, callCtx.Err()
		}
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "cancelled", CancellationID: "cancelled", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	if err := scheduler.Generate(ctx, descriptor, request, func(ProviderEvent) error { return nil }); !errors.Is(err, context.Canceled) || opens.Load() != 0 {
		t.Fatalf("pre-effect cancellation opened native worker: opens=%d err=%v", opens.Load(), err)
	}
	request.RequestID, request.CancellationID = "new-request", "new-request"
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil || opens.Load() != 1 || retireCalls.Load() != 2 {
		t.Fatalf("new request remained blocked after pre-effect cancellation: opens=%d retire=%d err=%v", opens.Load(), retireCalls.Load(), err)
	}
}

func TestProviderSchedulerDoesNotAdmitLegacyWhenNativeUnloadFails(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	fake := &schedulerFake{descriptor: descriptor, closeErr: errors.New("native unload failed")}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	if release, _, err := scheduler.AcquireGeneration(context.Background()); err == nil || release != nil {
		t.Fatalf("legacy load admitted after uncertain native unload: lease=%v err=%v", release != nil, err)
	}
	// A failed unload leaves resource ownership uncertain. Do not silently
	// assume a second admission is safe merely because map state was cleared.
	if release, _, err := scheduler.AcquireGeneration(context.Background()); err == nil || release != nil {
		t.Fatalf("legacy load resumed after unresolved native unload: lease=%v err=%v", release != nil, err)
	}
	// Neither a model switch nor a worker restart may bypass this boundary.
	if _, err := scheduler.Runtime(context.Background(), descriptor); err == nil {
		t.Fatal("native worker reopened after unresolved unload")
	}
}

func TestProviderSchedulerReusesResidentRuntimeAndClosesIt(t *testing.T) {
	registry := NewProviderRegistry()
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	fake := &schedulerFake{descriptor: descriptor}
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "r", CancellationID: "c", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"prompt":"hi"}`)}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if fake.generates.Load() != 2 {
		t.Fatalf("expected two generations, got %d", fake.generates.Load())
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	if fake.closes.Load() != 1 {
		t.Fatalf("expected one close, got %d", fake.closes.Load())
	}
}

func TestProviderSchedulerSwitchReleasesIdleResidentBeforeOpeningAnother(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	prior := &schedulerFake{descriptor: first}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if selected.ModelDigest == first.ModelDigest {
			return prior, nil
		}
		if prior.closes.Load() != 1 {
			return nil, errors.New("prior model is still resident")
		}
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	if _, err := scheduler.Runtime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Runtime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if prior.closes.Load() != 1 {
		t.Fatal("idle resident was not unloaded before switching")
	}
}

func TestProviderSchedulerDoesNotReplaceActiveResident(t *testing.T) {
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	worker := &blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: first}, started: make(chan struct{}, 1), release: make(chan struct{})}
	registry := NewProviderRegistry()
	var secondOpens atomic.Int32
	if err := registry.Register(ProviderMLX, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if selected.ModelDigest == first.ModelDigest {
			return worker, nil
		}
		secondOpens.Add(1)
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 2)
	defer scheduler.Close()
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Generate(context.Background(), first, ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: first.ModelDigest, Prompt: []byte(`{"messages":[]}`)}, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("first worker did not start")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := scheduler.Runtime(waitCtx, second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("active resident was not awaited: %v", err)
	}
	if worker.closes.Load() != 0 || secondOpens.Load() != 0 {
		t.Fatal("model switch displaced active inference")
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(worker.release)
	}()
	request := ProviderRequest{RequestID: "second", CancellationID: "second", ModelDigest: second.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	switchCtx, stopSwitch := context.WithTimeout(context.Background(), time.Second)
	defer stopSwitch()
	timing, switchErr := scheduler.GenerateTimed(switchCtx, second, request, func(ProviderEvent) error { return nil })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if switchErr != nil {
		t.Fatal(switchErr)
	}
	if timing.Queue < 20*time.Millisecond {
		t.Fatalf("active runner wait was not reported as queueing: %+v", timing)
	}
	if worker.closes.Load() != 1 || secondOpens.Load() != 1 {
		t.Fatal("safe boundary did not release the old runner before loading the new one")
	}
}

func TestProviderSchedulerSharedAdmissionQueuesLegacyAndNativeGeneration(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var loads atomic.Int32
	if err := registry.Register(ProviderTransformers, func(_ context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		loads.Add(1)
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	legacyRelease, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	request := ProviderRequest{RequestID: "native", CancellationID: "native", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	queued, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := scheduler.Generate(queued, descriptor, request, func(ProviderEvent) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("native generation did not wait behind legacy admission: %v", err)
	}
	if loads.Load() != 0 {
		t.Fatal("queued native request opened a worker")
	}
	legacyRelease()
	legacyRelease() // A caller must not free another generation's slot.
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 1 {
		t.Fatalf("native worker was not admitted after legacy release: %d", loads.Load())
	}
}

func TestProviderSchedulerRejectsMismatchedModel(t *testing.T) {
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return &schedulerFake{}, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	request := ProviderRequest{RequestID: "r", CancellationID: "c", ModelDigest: "sha256:other", Prompt: []byte(`{"prompt":"hi"}`)}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("expected model mismatch")
	}
}

func TestProviderSchedulerRejectsContextBeyondSelectedRuntimeBeforeLoad(t *testing.T) {
	registry := NewProviderRegistry()
	var loads atomic.Int32
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		loads.Add(1)
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", ContextLength: 4096, Capabilities: []ProviderCapability{CapabilityText}}
	request := ProviderRequest{RequestID: "r", CancellationID: "c", ModelDigest: descriptor.ModelDigest, ContextLength: 8192, Prompt: []byte(`{"messages":[]}`)}
	if err := scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("oversize context was admitted")
	}
	if loads.Load() != 0 {
		t.Fatal("provider opened before context validation")
	}
}

func TestProviderSchedulerRejectsNilRuntimeWithoutPanicking(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	if _, err := scheduler.Runtime(context.Background(), descriptor); err == nil {
		t.Fatal("accepted an absent provider runtime")
	}
}

func TestProviderSchedulerDeduplicatesConcurrentLoadsAndCancelDoesNotLoad(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	var loads atomic.Int32
	fake := &schedulerFake{descriptor: descriptor}
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) {
		loads.Add(1)
		time.Sleep(30 * time.Millisecond)
		return fake, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Cancel(context.Background(), descriptor, "cancel-before-load"); err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 0 {
		t.Fatal("Cancel started a provider")
	}
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := scheduler.Runtime(context.Background(), descriptor); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if loads.Load() != 1 {
		t.Fatalf("expected one provider load, got %d", loads.Load())
	}
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
}

type blockingSchedulerRuntime struct {
	schedulerFake
	started chan struct{}
	release chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

type cancellableSchedulerRuntime struct {
	blockingSchedulerRuntime
	cancelled chan string
}

func (f *cancellableSchedulerRuntime) Cancel(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.cancelled <- id
	return nil
}

type unhealthyWhileActiveRuntime struct {
	blockingSchedulerRuntime
	unhealthy atomic.Bool
}

func (f *unhealthyWhileActiveRuntime) Health(context.Context) error {
	if f.unhealthy.Load() {
		return errors.New("worker became unhealthy")
	}
	return nil
}

func TestProviderSchedulerDoesNotCloseActiveUnhealthyRuntime(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	worker := &unhealthyWhileActiveRuntime{blockingSchedulerRuntime: blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 1), release: make(chan struct{})}}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderMLX, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return worker, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("generation did not start")
	}
	worker.unhealthy.Store(true)
	if _, err := scheduler.Runtime(context.Background(), descriptor); err == nil {
		t.Fatal("unhealthy active worker was reused")
	}
	if worker.closes.Load() != 0 {
		t.Fatal("active worker was closed during generation")
	}
	close(worker.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func (f *blockingSchedulerRuntime) Generate(ctx context.Context, request ProviderRequest, callback func(ProviderEvent) error) error {
	current := f.active.Add(1)
	for {
		peak := f.maximum.Load()
		if current <= peak || f.maximum.CompareAndSwap(peak, current) {
			break
		}
	}
	f.started <- struct{}{}
	select {
	case <-f.release:
	case <-ctx.Done():
		f.active.Add(-1)
		return ctx.Err()
	}
	f.active.Add(-1)
	return callback(ProviderEvent{ProtocolRevision: f.descriptor.ProtocolRevision, Type: ProviderEventDone, RequestID: request.RequestID, Sequence: 1, FinishReason: "stop"})
}

func TestProviderSchedulerSerializesUnqualifiedGenerationAndCancelsQueuedRequest(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	fake := &blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 2), release: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	first := ProviderRequest{RequestID: "first", CancellationID: "first", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	second := ProviderRequest{RequestID: "second", CancellationID: "second", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- scheduler.Generate(context.Background(), descriptor, first, func(ProviderEvent) error { return nil })
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("first generation did not start")
	}
	queued, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- scheduler.Generate(queued, descriptor, second, func(ProviderEvent) error { return nil })
	}()
	cancel()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued request was not cancelled: %v", err)
	}
	close(fake.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if fake.maximum.Load() != 1 {
		t.Fatalf("unqualified hardware ran %d generations concurrently", fake.maximum.Load())
	}
}

func TestProviderSchedulerSeparatesNativeAndLegacyFamiliesWithMultipleSlots(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	fake := &blockingSchedulerRuntime{schedulerFake: schedulerFake{descriptor: descriptor}, started: make(chan struct{}, 2), release: make(chan struct{})}
	registry := NewProviderRegistry()
	if err := registry.Register(ProviderTransformers, func(context.Context, ProviderDescriptor) (ProviderRuntime, error) { return fake, nil }); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	request := func(id string) ProviderRequest {
		return ProviderRequest{RequestID: id, CancellationID: id, ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- scheduler.Generate(context.Background(), descriptor, request("first"), func(ProviderEvent) error { return nil })
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("first native request did not start")
	}
	type outcome struct {
		timing ProviderTiming
		err    error
	}
	secondDone := make(chan outcome, 1)
	go func() {
		timing, err := scheduler.GenerateTimed(context.Background(), descriptor, request("second"), func(ProviderEvent) error { return nil })
		secondDone <- outcome{timing, err}
	}()
	waitForScheduledRequest(t, scheduler, descriptor, "second")
	legacyCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	legacyRelease, wait, err := scheduler.AcquireGeneration(legacyCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || legacyRelease != nil || wait <= 0 {
		t.Fatalf("legacy inference overlapped an unqualified native worker: lease=%v wait=%v err=%v", legacyRelease != nil, wait, err)
	}
	if err := scheduler.Cancel(context.Background(), descriptor, "second"); err != nil {
		t.Fatal(err)
	}
	secondResult := <-secondDone
	if !errors.Is(secondResult.err, context.Canceled) || secondResult.timing.Queue <= 0 {
		t.Fatalf("queued native request ignored Stop or lost wait timing: %+v", secondResult)
	}
	close(fake.release)
	if err := <-firstDone; err != nil || fake.maximum.Load() != 1 || fake.generates.Load() != 0 {
		t.Fatalf("native workers overlapped or failed: err=%v maximum=%d generations=%d", err, fake.maximum.Load(), fake.generates.Load())
	}
	legacyRelease, _, err = scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatalf("legacy inference remained blocked after native completion: %v", err)
	}
	legacyRelease()
}

func TestProviderSchedulerNativeWaiterPreventsLegacyStarvation(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	firstLegacy, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer firstLegacy()
	request := ProviderRequest{RequestID: "waiting", CancellationID: "waiting", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	nativeDone := make(chan error, 1)
	go func() {
		nativeDone <- scheduler.Generate(context.Background(), descriptor, request, func(ProviderEvent) error { return nil })
	}()
	deadline := time.Now().Add(time.Second)
	for {
		scheduler.mu.Lock()
		waiting := scheduler.nativeWait
		scheduler.mu.Unlock()
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("native request did not enter the family queue")
		}
		time.Sleep(time.Millisecond)
	}
	legacyCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	secondLegacy, _, err := scheduler.AcquireGeneration(legacyCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || secondLegacy != nil {
		t.Fatalf("later legacy work bypassed a waiting native request: %v", err)
	}
	if err := scheduler.Cancel(context.Background(), descriptor, request.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-nativeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("native waiter did not cancel: %v", err)
	}
	secondLegacy, _, err = scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatalf("legacy lane did not reopen after cancellation: %v", err)
	}
	secondLegacy()
}

func TestProviderSchedulerSerializesDifferentModelLoads(t *testing.T) {
	registry := NewProviderRegistry()
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	started := make(chan string, 2)
	release := make(chan struct{})
	var active, maximum atomic.Int32
	if err := registry.Register(ProviderMLX, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		current := active.Add(1)
		for {
			peak := maximum.Load()
			if current <= peak || maximum.CompareAndSwap(peak, current) {
				break
			}
		}
		started <- descriptor.ModelDigest
		<-release
		active.Add(-1)
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	results := make(chan error, 2)
	go func() { _, err := scheduler.Runtime(context.Background(), first); results <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first model did not start loading")
	}
	go func() { _, err := scheduler.Runtime(context.Background(), second); results <- err }()
	select {
	case <-started:
		t.Fatal("different model loaded while first load was in progress")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if maximum.Load() != 1 {
		t.Fatalf("expected one concurrent model load, got %d", maximum.Load())
	}
}

func TestProviderSchedulerCancelledLoadWaiterDoesNotOpenWorker(t *testing.T) {
	registry := NewProviderRegistry()
	first := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:first", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	second := first
	second.ModelDigest = "sha256:second"
	started := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32
	if err := registry.Register(ProviderTransformers, func(_ context.Context, descriptor ProviderDescriptor) (ProviderRuntime, error) {
		loads.Add(1)
		if descriptor.ModelDigest == first.ModelDigest {
			close(started)
			<-release
		}
		return &schedulerFake{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	firstDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), first); firstDone <- err }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(ctx, second); secondDone <- err }()
	cancel()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled load returned %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 1 {
		t.Fatalf("cancelled load opened a worker: %d loads", loads.Load())
	}
}

func TestProviderSchedulerWaiterRecoversFromCancelledLoadLeader(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderTransformers, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatSafetensors, Backend: "cpu", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	started := make(chan struct{})
	var loads atomic.Int32
	if err := registry.Register(ProviderTransformers, func(ctx context.Context, selected ProviderDescriptor) (ProviderRuntime, error) {
		if loads.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &schedulerFake{descriptor: selected}, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	defer scheduler.Close()
	leaderCtx, cancel := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(leaderCtx, descriptor); leaderDone <- err }()
	<-started
	waiterDone := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); waiterDone <- err }()
	cancel()
	if err := <-leaderDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader returned %v", err)
	}
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatalf("live waiter inherited the cancelled leader: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not retry after cancelled load")
	}
	if loads.Load() != 2 {
		t.Fatalf("expected one new load after cancellation, got %d", loads.Load())
	}
}

func TestProviderSchedulerCloseCancelsPendingWorkerLoad(t *testing.T) {
	descriptor := ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderMLX, RunnerRevision: "r1", AdapterRevision: "a1", ModelDigest: "sha256:model", Format: FormatMLXSafetensors, Backend: "metal", Capabilities: []ProviderCapability{CapabilityText}}
	registry := NewProviderRegistry()
	started := make(chan struct{})
	if err := registry.Register(ProviderMLX, func(ctx context.Context, _ ProviderDescriptor) (ProviderRuntime, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, _ := NewProviderScheduler(registry, 1)
	result := make(chan error, 1)
	go func() { _, err := scheduler.Runtime(context.Background(), descriptor); result <- err }()
	<-started
	if err := scheduler.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("closed scheduler accepted a pending worker")
		}
	case <-time.After(time.Second):
		t.Fatal("closing scheduler did not cancel worker startup")
	}
}

func TestProviderSchedulerCoordinatesLegacyModelLoadReuseAndInvalidation(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	gate := NewLifecycleGate(1)
	loads := 0
	alive := true
	loader := func(context.Context, string) error {
		loads++
		time.Sleep(5 * time.Millisecond)
		return nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt == 2 {
			alive = false
		}
		release, timing, err := scheduler.AcquireLegacyModel(context.Background(), gate, "model.gguf", 4096, func() bool { return alive }, loader)
		if err != nil {
			t.Fatal(err)
		}
		if attempt != 1 && timing.Load <= 0 || attempt == 1 && timing.Load != 0 {
			t.Fatalf("legacy load timing did not distinguish reuse: attempt=%d timing=%+v", attempt, timing)
		}
		release()
	}
	if loads != 2 {
		t.Fatalf("legacy model loaded %d times, expected initial load and one dead-process reload", loads)
	}
}

func TestProviderSchedulerRejectsLegacyModelWithoutLifecycleGate(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	called := false
	release, _, err := scheduler.AcquireLegacyModel(context.Background(), nil, "model.gguf", 4096, nil, func(context.Context, string) error { called = true; return nil })
	if err == nil || release != nil || called {
		t.Fatalf("legacy model switched without lifecycle protection: lease=%v called=%v err=%v", release != nil, called, err)
	}
}

func TestProviderSchedulerLegacyLoadFailureReleasesAdmission(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	gate := NewLifecycleGate(1)
	failure := errors.New("model load failed")
	release, _, err := scheduler.AcquireLegacyModel(context.Background(), gate, "model.gguf", 4096, nil, func(context.Context, string) error { return failure })
	if release != nil || !errors.Is(err, failure) {
		t.Fatalf("failed load returned a generation lease: release=%v err=%v", release != nil, err)
	}
	ready, _, err := scheduler.AcquireLegacyModel(context.Background(), gate, "model.gguf", 4096, nil, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatalf("failed load leaked admission: %v", err)
	}
	ready()
}

func TestProviderSchedulerLegacyLoadWaitsForNativeFamilyWithoutDispatch(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	releaseNative, _, err := scheduler.acquireFamily(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseNative()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	loaded := false
	release, _, err := scheduler.AcquireLegacyModel(ctx, NewLifecycleGate(1), "model.gguf", 4096, nil, func(context.Context, string) error { loaded = true; return nil })
	if release != nil || !errors.Is(err, context.DeadlineExceeded) || loaded {
		t.Fatalf("legacy load crossed native admission: lease=%v loaded=%v err=%v", release != nil, loaded, err)
	}
}

func TestProviderSchedulerLegacyMaintenanceIsExclusiveAcrossParallelSlots(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	active, _, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if release, _, err := scheduler.AcquireLegacyMaintenance(ctx); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("maintenance overlapped active llama generation: lease=%v err=%v", release != nil, err)
	}
	active()
	maintenance, _, err := scheduler.AcquireLegacyMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if release, _, err := scheduler.AcquireGenerationTimed(ctx); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("llama generation overlapped process maintenance: lease=%v err=%v", release != nil, err)
	}
	maintenance()
	ready, _, err := scheduler.AcquireGenerationTimed(context.Background())
	if err != nil {
		t.Fatalf("released maintenance leaked admission: %v", err)
	}
	ready()
}
