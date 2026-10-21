package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type nativeChatSelectorFixture struct {
	descriptor inference.ProviderDescriptor
	err        error
}

func TestNativeTextInstallationRequiresPinnedWorkersAndEvidenceTarget(t *testing.T) {
	root := t.TempDir()
	registry := inference.NewProviderRegistry()
	scheduler, err := inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	s := &Server{config: &config.Config{DataDir: t.TempDir()}, registry: models.NewRegistry(t.TempDir()), providerRegistry: registry, providerScheduler: scheduler}
	worker := inference.TrustedNativeWorker{Provider: inference.ProviderTransformers, RunnerRevision: "fixture", PythonPath: filepath.Join(root, "python"), ScriptPath: filepath.Join(root, "worker.py"), DependencyVersions: map[string]string{"torch": "1.0", "transformers": "1.0", "safetensors": "1.0"}}
	for _, path := range []string{worker.PythonPath, worker.ScriptPath} {
		data := []byte("isolated worker fixture: " + path)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if path == worker.PythonPath {
			worker.PythonSHA256 = "sha256:" + hex.EncodeToString(sum[:])
		} else {
			worker.ScriptSHA256 = "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	mlxWorker := worker
	mlxWorker.Provider = inference.ProviderMLX
	mlxWorker.DependencyVersions = map[string]string{"mlx": "1.0", "mlx-lm": "1.0", "transformers": "1.0"}
	workers := map[inference.ProviderID]inference.TrustedNativeWorker{inference.ProviderTransformers: worker, inference.ProviderMLX: mlxWorker}
	if err := s.InstallQualifiedNativeText(context.Background(), models.NewAdapterRegistry(), workers, inference.NativeTextEvidenceTarget{}); err == nil {
		t.Fatal("incomplete qualification target installed")
	}
	target := inference.NativeTextEvidenceTarget{HardwareID: "fixture-cpu", Backend: "cpu", Platform: "windows-test", SuiteSHA256: strings.Repeat("a", 64)}
	metalTarget := inference.NativeTextEvidenceTarget{HardwareID: "fixture-metal", Backend: "metal", Platform: "darwin-test", SuiteSHA256: strings.Repeat("b", 64)}
	targets := map[inference.NativeTextTargetKey]inference.NativeTextEvidenceTarget{
		{Provider: inference.ProviderTransformers, Backend: "cpu"}: target,
		{Provider: inference.ProviderMLX, Backend: "metal"}:        metalTarget,
	}
	if err := s.InstallQualifiedNativeTextTargets(context.Background(), models.NewAdapterRegistry(), map[inference.ProviderID]inference.TrustedNativeWorker{inference.ProviderTransformers: worker}, targets); err == nil {
		t.Fatal("qualification target without an installed worker was accepted")
	}
	if err := s.InstallQualifiedNativeTextTargets(context.Background(), models.NewAdapterRegistry(), workers, map[inference.NativeTextTargetKey]inference.NativeTextEvidenceTarget{{Provider: inference.ProviderTransformers, Backend: "cpu"}: target}); err == nil {
		t.Fatal("installed native worker without a qualification target was accepted")
	}
	if err := s.InstallQualifiedNativeTextTargets(context.Background(), models.NewAdapterRegistry(), workers, targets); err != nil {
		t.Fatal(err)
	}
	if err := s.InstallQualifiedNativeText(context.Background(), models.NewAdapterRegistry(), workers, target); err == nil {
		t.Fatal("duplicate native factory installation was accepted")
	}
	if _, selected, err := s.selectNativeChat(context.Background(), "missing@r1"); !selected || err == nil {
		t.Fatal("installed routing promoted a missing package")
	}
}

func (f nativeChatSelectorFixture) Select(context.Context, string, string) (inference.ProviderDescriptor, error) {
	return f.descriptor, f.err
}

type nativeChatRuntimeFixture struct {
	descriptor   inference.ProviderDescriptor
	called       int
	incomplete   bool
	finishReason string
}

func (f *nativeChatRuntimeFixture) Descriptor(context.Context) (inference.ProviderDescriptor, error) {
	return f.descriptor, nil
}
func (f *nativeChatRuntimeFixture) Resources(context.Context) (inference.ProviderResources, error) {
	return inference.ProviderResources{Backend: f.descriptor.Backend, Device: "isolated fixture"}, nil
}
func (f *nativeChatRuntimeFixture) Health(context.Context) error         { return nil }
func (f *nativeChatRuntimeFixture) Close() error                         { return nil }
func (f *nativeChatRuntimeFixture) Cancel(context.Context, string) error { return nil }
func (f *nativeChatRuntimeFixture) Generate(_ context.Context, req inference.ProviderRequest, emit func(inference.ProviderEvent) error) error {
	f.called++
	if err := emit(inference.ProviderEvent{ProtocolRevision: inference.ProviderProtocolRevision, Type: inference.ProviderEventDelta, RequestID: req.RequestID, Sequence: 1, Text: "fixture answer"}); err != nil {
		return err
	}
	if f.incomplete {
		return nil
	}
	if err := emit(inference.ProviderEvent{ProtocolRevision: inference.ProviderProtocolRevision, Type: inference.ProviderEventUsage, RequestID: req.RequestID, Sequence: 2, Usage: &inference.UsageMetrics{PromptTokens: 2, CompletionTokens: 2, TotalTokens: 4}}); err != nil {
		return err
	}
	reason := f.finishReason
	if reason == "" {
		reason = "stop"
	}
	return emit(inference.ProviderEvent{ProtocolRevision: inference.ProviderProtocolRevision, Type: inference.ProviderEventDone, RequestID: req.RequestID, Sequence: 3, FinishReason: reason})
}

func TestNativeChatRequiresServiceOwnedQualificationAndUsesSharedScheduler(t *testing.T) {
	descriptor := inference.ProviderDescriptor{ProtocolRevision: inference.ProviderProtocolRevision, Provider: inference.ProviderTransformers, RunnerRevision: "fixture", AdapterRevision: "fixture", ModelDigest: "sha256:fixture", Format: inference.FormatSafetensors, Backend: "cpu", Capabilities: []inference.ProviderCapability{inference.CapabilityText, inference.CapabilityStreaming}}
	registry := inference.NewProviderRegistry()
	runtime := &nativeChatRuntimeFixture{descriptor: descriptor}
	if err := registry.Register(inference.ProviderTransformers, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
		return runtime, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	s := &Server{registry: models.NewRegistry(t.TempDir()), providerScheduler: scheduler}
	request := &api.ChatCompletionRequest{Model: "test@revision", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}}
	if _, err := s.completeChat(context.Background(), request); err == nil {
		t.Fatal("native package was selectable without a service-owned selector")
	}
	s.nativeChatSelector = nativeChatSelectorFixture{descriptor: descriptor, err: errors.New("unqualified")}
	if _, err := s.completeChat(context.Background(), request); err == nil || runtime.called != 0 {
		t.Fatal("unqualified native package reached inference")
	}
	s.nativeChatSelector = nativeChatSelectorFixture{descriptor: descriptor}
	response, err := s.completeChat(context.Background(), request)
	if err != nil || response == nil || len(response.Choices) != 1 || response.Choices[0].Message.Content != "fixture answer" || runtime.called != 1 {
		t.Fatalf("qualified native Chat did not use shared scheduler: response=%+v called=%d err=%v", response, runtime.called, err)
	}
	var events []ChatStreamEvent
	streamed, err := s.streamSessionChat(context.Background(), request, "interactive", func(event ChatStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil || streamed.Answer != "fixture answer" || streamed.FinishReason != "stop" {
		t.Fatalf("native session stream failed: %+v, err=%v", streamed, err)
	}
	seenDelta := false
	for _, event := range events {
		seenDelta = seenDelta || event.Type == "delta" && event.Delta == "fixture answer"
	}
	if !seenDelta {
		t.Fatalf("native session did not stream text: %+v", events)
	}
	runtime.finishReason = "length"
	limited, err := s.completeChat(context.Background(), request)
	if err != nil || limited.Choices[0].FinishReason != "length" {
		t.Fatalf("native Chat lost token-limit finish reason: %+v, err=%v", limited, err)
	}
	limitedStream, err := s.streamSessionChat(context.Background(), request, "interactive", func(ChatStreamEvent) error { return nil })
	if err != nil || limitedStream.FinishReason != "length" {
		t.Fatalf("native session lost token-limit finish reason: %+v, err=%v", limitedStream, err)
	}
	streamRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test@revision","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	streamResponse := httptest.NewRecorder()
	s.handleChatCompletions(streamResponse, streamRequest)
	if streamResponse.Code != http.StatusOK || !strings.Contains(streamResponse.Body.String(), "fixture answer") || !strings.Contains(streamResponse.Body.String(), "data: [DONE]") {
		t.Fatalf("native OpenAI stream failed: status=%d body=%s", streamResponse.Code, streamResponse.Body.String())
	}
	if !strings.Contains(streamResponse.Body.String(), `"finish_reason":"length"`) {
		t.Fatalf("native OpenAI stream lost token-limit finish reason: %s", streamResponse.Body.String())
	}
	runtime.finishReason = ""
	runtime.incomplete = true
	brokenResponse := httptest.NewRecorder()
	s.handleChatCompletions(brokenResponse, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"test@revision","messages":[{"role":"user","content":"hello"}],"stream":true}`)))
	if !strings.Contains(brokenResponse.Body.String(), "native_inference_failed") || strings.Contains(brokenResponse.Body.String(), "data: [DONE]") {
		t.Fatalf("truncated native stream was marked complete: %s", brokenResponse.Body.String())
	}
}
