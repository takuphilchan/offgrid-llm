//go:build !llama

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAgentStreamReportsConstrainedToolActivityBeforeCompletion(t *testing.T) {
	release := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": `{"tool_call":{"name":"fixture","arguments":{"private":"`}}}})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer backend.Close()
	gate := inference.NewLifecycleGate(1)
	unlock, err := gate.AcquireWithContext(context.Background(), "fixture", 4096, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	s := &Server{engine: inference.NewLlamaHTTPEngine(backend.URL), inferenceLifecycle: gate}
	task := &agents.Task{Model: "fixture", Config: agents.AgentConfig{TaskFirst: true, ContextWindow: 4096, MaxTokens: 32}}
	tools := []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "fixture", Parameters: map[string]any{"type": "object"}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	activity := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		response, err := s.streamAgentModel(ctx, task, []api.ChatMessage{{Role: "user", Content: "test"}}, tools, func(phase, text string) error {
			if text != "" {
				t.Error("private tool proposal exposed as text")
			}
			if phase == "preparing_tool" {
				activity <- struct{}{}
			}
			return nil
		})
		if response != nil {
			t.Error("incomplete proposal returned as a decision")
		}
		done <- err
	}()
	select {
	case <-activity:
	case <-ctx.Done():
		t.Error("tool preparation did not reach agent progress")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("truncated response accepted")
	}
}
