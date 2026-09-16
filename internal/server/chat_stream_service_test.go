package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type chatServiceStreamEngine struct {
	inference.Engine
	finish  string
	failure error
	calls   int
}

func (e *chatServiceStreamEngine) ChatCompletionStreamRaw(ctx context.Context, req *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	e.calls++
	if !req.Stream || req.StreamOptions == nil || !req.StreamOptions.IncludeUsage {
		return errors.New("missing streaming usage options")
	}
	if err := callback(json.RawMessage(`{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`)); err != nil {
		return err
	}
	if e.failure != nil {
		return e.failure
	}
	terminal, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{}, "finish_reason": e.finish}}, "usage": map[string]int{"completion_tokens": 4}})
	return callback(terminal)
}

func TestStreamChatServiceCompletionAndFailures(t *testing.T) {
	for _, test := range []struct {
		name, finish string
		failure      error
		knowledge    bool
		wantSuccess  bool
	}{
		{"complete", "stop", nil, false, true},
		{"budget reached", "length", nil, false, true},
		{"missing finish", "", nil, false, false},
		{"partial failure", "", io.ErrUnexpectedEOF, false, false},
		{"knowledge unavailable", "stop", nil, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			registry := models.NewRegistry(dir)
			if err := registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			gate := inference.NewLifecycleGate(1)
			release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			release()
			engine := &chatServiceStreamEngine{finish: test.finish, failure: test.failure}
			s := &Server{config: &config.Config{MaxContextSize: 65536, ChatContextSize: 8192}, registry: registry, engine: engine, inferenceLifecycle: gate}
			var events []ChatStreamEvent
			result, err := s.streamSessionChat(context.Background(), &api.ChatCompletionRequest{Model: "model", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, Stream: true, UseKnowledgeBase: &test.knowledge}, "interactive", func(event ChatStreamEvent) error { events = append(events, event); return nil })
			if (err == nil) != test.wantSuccess {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			if test.wantSuccess && (result.Answer != "Hello" || result.Metrics.CompletionTokens != 4 || result.Metrics.ContextWindow != 8192 || result.FinishReason != test.finish) {
				t.Fatalf("wrong result: %+v", result)
			}
			if test.knowledge && engine.calls != 0 {
				t.Fatal("inference ran without required knowledge")
			}
			if gate.Status().Active != 0 {
				t.Fatal("inference lease leaked")
			}
		})
	}
}
