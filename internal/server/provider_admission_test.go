package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestLegacyInferenceWaitsForSharedProviderAdmissionBeforeLoading(t *testing.T) {
	scheduler, err := inference.NewProviderScheduler(inference.NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{providerScheduler: scheduler, inferenceLifecycle: inference.NewLifecycleGate(1)}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if release, err := s.acquireInferenceContext(ctx, "legacy", 4096, nil); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("legacy inference bypassed provider queue: release=%v err=%v", release != nil, err)
	}
	held()
	// Admission is reusable after a cancelled legacy waiter; it did not leave
	// a ghost lease that would block a later generation.
	ready, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ready()
}

func TestModelLoadTimeoutStartsAfterProviderQueue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "queued.gguf"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	scheduler, err := inference.NewProviderScheduler(inference.NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	s := &Server{config: &config.Config{ModelsDir: dir, ModelLoadTimeout: 1}, registry: registry,
		llamaProcess: &llamaLifecycleFixture{}, inferenceLifecycle: inference.NewLifecycleGate(1), providerScheduler: scheduler}
	type outcome struct {
		release func()
		timing  inference.ProviderTiming
		err     error
	}
	started := make(chan struct{})
	finished := make(chan outcome, 1)
	go func() {
		close(started)
		release, timing, err := s.acquireInferenceContextTimed(context.Background(), "queued", 4096, nil)
		finished <- outcome{release: release, timing: timing, err: err}
	}()
	<-started
	time.Sleep(1500 * time.Millisecond)
	held()
	select {
	case got := <-finished:
		if got.release != nil {
			defer got.release()
		}
		if got.err != nil || got.release == nil || got.timing.Queue < time.Second {
			t.Fatalf("queue wait consumed model load timeout: timing=%+v err=%v", got.timing, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued model did not finish after admission")
	}
}

type blockingLlamaLifecycle struct{}

func (*blockingLlamaLifecycle) Activate(ctx context.Context, _, _, _ string, _ inference.LoadOptions) (int, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}
func (*blockingLlamaLifecycle) Alive(string) bool                       { return false }
func (*blockingLlamaLifecycle) Generation(string) uint64                { return 0 }
func (*blockingLlamaLifecycle) RetireAll(context.Context) (bool, error) { return false, nil }

func TestModelLoadTimeoutStillBoundsActualActivation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "slow.gguf"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{ModelsDir: dir, ModelLoadTimeout: 1}, registry: registry,
		llamaProcess: &blockingLlamaLifecycle{}, inferenceLifecycle: inference.NewLifecycleGate(1)}
	release, timing, err := s.acquireInferenceContextTimed(context.Background(), "slow", 4096, nil)
	if release != nil || !errors.Is(err, context.DeadlineExceeded) || timing.Load < 900*time.Millisecond {
		t.Fatalf("actual model activation escaped its timeout: release=%v timing=%+v err=%v", release != nil, timing, err)
	}
}

func TestRetireLegacyForNativeResetsEngineAndLifecycle(t *testing.T) {
	engine := inference.NewMockEngine()
	if err := engine.Load(context.Background(), "fixture.gguf", inference.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "legacy", 4096, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	s := &Server{engine: engine, inferenceLifecycle: gate, currentModelID: "legacy", currentContext: 4096, currentPort: 12345}
	retired, err := s.retireLegacyForNative(context.Background())
	if err != nil || !retired || engine.IsLoaded() || s.currentModelID != "" || s.currentContext != 0 || s.currentPort != 0 || gate.Status().CurrentModel != "" {
		t.Fatalf("legacy residency was not cleared: retired=%v err=%v model=%q context=%d port=%d gate=%+v", retired, err, s.currentModelID, s.currentContext, s.currentPort, gate.Status())
	}
}

func TestRetireLegacyForNativeCancelledBeforeEffects(t *testing.T) {
	engine := inference.NewMockEngine()
	if err := engine.Load(context.Background(), "fixture.gguf", inference.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	s := &Server{engine: engine, currentModelID: "legacy", currentPort: 12345}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	retired, err := s.retireLegacyForNative(ctx)
	if !errors.Is(err, context.Canceled) || retired || !engine.IsLoaded() || s.currentModelID != "legacy" {
		t.Fatalf("cancelled retirement changed llama state: retired=%v err=%v", retired, err)
	}
}

func TestRetireLegacyForNativeStopWhileModelLockHeld(t *testing.T) {
	engine := inference.NewMockEngine()
	if err := engine.Load(context.Background(), "fixture.gguf", inference.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	s := &Server{engine: engine, currentModelID: "legacy"}
	s.modelMutex.Lock()
	defer s.modelMutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := s.retireLegacyForNative(ctx)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) || !engine.IsLoaded() || s.currentModelID != "legacy" {
			t.Fatalf("Stop did not cancel the mutex wait safely: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("native retirement ignored cancellation while waiting for model lock")
	}
}

func TestServerNativeAdmissionRetiresLoadedLlamaEngine(t *testing.T) {
	s := newTestServer(t)
	if err := s.engine.Load(context.Background(), "fixture.gguf", inference.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	s.currentModelID = "legacy"
	s.currentPort = 12345
	descriptor := inference.ProviderDescriptor{ProtocolRevision: inference.ProviderProtocolRevision, Provider: inference.ProviderTransformers, RunnerRevision: "fixture", AdapterRevision: "fixture", ModelDigest: "sha256:fixture", Format: inference.FormatSafetensors, Backend: "cpu", Capabilities: []inference.ProviderCapability{inference.CapabilityText}}
	if err := s.providerRegistry.Register(inference.ProviderTransformers, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
		if s.engine.IsLoaded() || s.currentModelID != "" {
			return nil, errors.New("native worker opened before llama engine retirement")
		}
		return &nativeChatRuntimeFixture{descriptor: descriptor}, nil
	}); err != nil {
		t.Fatal(err)
	}
	request := inference.ProviderRequest{RequestID: "native", CancellationID: "native", ModelDigest: descriptor.ModelDigest, Prompt: []byte(`{"messages":[]}`)}
	if err := s.providerScheduler.Generate(context.Background(), descriptor, request, func(inference.ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if s.engine.IsLoaded() || s.currentPort != 0 {
		t.Fatal("native admission retained the loaded llama engine")
	}
}

func TestMissingSchedulerOwnedLlamaHandleFailsClosed(t *testing.T) {
	s := newTestServer(t)
	modelPath := filepath.Join(s.config.ModelsDir, "owned-fixture.gguf")
	if err := os.WriteFile(modelPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	s.llamaProcess = nil
	if err := s.switchModelWithContextSize(context.Background(), "owned-fixture", 4096); err == nil || !strings.Contains(err.Error(), "scheduler-owned") {
		t.Fatalf("model activation rebuilt an unowned llama handle: %v", err)
	}
	if s.modelCache.IsModelAlive("owned-fixture") {
		t.Fatal("rejected activation launched a llama process")
	}
	if err := s.engine.Load(context.Background(), modelPath, inference.LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	retired, err := s.retireLegacyForNative(context.Background())
	if retired || err == nil || !strings.Contains(err.Error(), "scheduler-owned") || !s.engine.IsLoaded() {
		t.Fatalf("native switching rebuilt an unowned llama handle: retired=%v loaded=%v err=%v", retired, s.engine.IsLoaded(), err)
	}
}

func TestHotSwapCannotLoadOutsideProviderAdmission(t *testing.T) {
	s := newTestServer(t)
	modelPath := filepath.Join(s.config.ModelsDir, "swap-fixture.gguf")
	if err := os.WriteFile(modelPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	held, _, err := s.providerScheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/models/hot-swap", strings.NewReader(`{"model_id":"swap-fixture"}`)).WithContext(ctx)
	s.handleHotSwap(w, req)
	if !strings.Contains(w.Body.String(), `"success":false`) || s.modelCache.IsModelAlive("swap-fixture") {
		t.Fatalf("hot-swap bypassed shared admission: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteModelCannotBypassProviderAdmission(t *testing.T) {
	s := newTestServer(t)
	modelPath := filepath.Join(s.config.ModelsDir, "delete-fixture.gguf")
	if err := os.WriteFile(modelPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	held, _, err := s.providerScheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/models", strings.NewReader(`{"model_id":"delete-fixture"}`)).WithContext(ctx)
	s.handleDeleteModel(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete bypassed provider admission: status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("blocked deletion changed model file: %v", err)
	}
}

func TestDeleteModelThroughProviderOwnerPreservesResponseAndRemovesFixture(t *testing.T) {
	s := newTestServer(t)
	modelPath := filepath.Join(s.config.ModelsDir, "delete-success-fixture.gguf")
	if err := os.WriteFile(modelPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/v1/models", strings.NewReader(`{"model_id":"delete-success-fixture"}`))
	s.handleDeleteModel(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("provider-owned deletion changed response: status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(modelPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("provider-owned deletion left model file: %v", err)
	}
	if _, err := s.registry.GetChatModel("delete-success-fixture"); err == nil {
		t.Fatal("provider-owned deletion left registered model")
	}
}
