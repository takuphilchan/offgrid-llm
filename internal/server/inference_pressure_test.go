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

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/degradation"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestInferenceBusyIsTypedAcrossChatTransportsAndReleasesSlots(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	manager := degradation.NewManager(degradation.DefaultConfig())
	defer manager.Stop()
	for i := 0; i < manager.MaxConcurrentRequests(); i++ {
		if !manager.RequestStart() {
			t.Fatal("fixture admission failed")
		}
	}
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	engine := &chatServiceStreamEngine{finish: "stop"}
	s := &Server{config: &config.Config{MaxContextSize: 65536, ChatContextSize: 8192}, registry: registry, engine: engine, inferenceLifecycle: gate, degradationMgr: manager}
	request := &api.ChatCompletionRequest{Model: "model", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, Stream: true}
	check := func(err error) {
		t.Helper()
		var typed *serviceError
		if !errors.As(err, &typed) || typed.code != "inference_busy" {
			t.Fatalf("expected typed busy rejection: %v", err)
		}
	}
	_, err = s.completeChat(context.Background(), request)
	check(err)
	_, err = s.streamSessionChat(context.Background(), request, "interactive", func(ChatStreamEvent) error { t.Fatal("rejected request emitted inference progress"); return nil })
	check(err)
	response := httptest.NewRecorder()
	s.handleChatCompletions(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"model","messages":[{"role":"user","content":"hello"}],"stream":true}`)))
	if response.Code != 503 || !strings.Contains(response.Body.String(), `"code":"inference_busy"`) {
		t.Fatalf("untyped HTTP rejection: %d %s", response.Code, response.Body.String())
	}
	if engine.calls != 0 {
		t.Fatal("rejected request reached inference")
	}
	for i := 0; i < manager.MaxConcurrentRequests(); i++ {
		manager.RequestEnd()
	}
	for i := 0; i < 3; i++ {
		result, err := s.streamSessionChat(context.Background(), request, "interactive", func(ChatStreamEvent) error { return nil })
		if err != nil || result.Answer != "Hello" {
			t.Fatalf("request did not recover: %+v %v", result, err)
		}
	}
	if engine.calls != 3 || gate.Status().Active != 0 {
		t.Fatal("admission or inference lease leaked")
	}
}
