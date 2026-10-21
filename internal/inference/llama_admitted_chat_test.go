package inference

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAdmittedLlamaChatUsesProviderTerminalGate(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		chunks   []json.RawMessage
		failure  error
		complete bool
	}{
		{"complete", []json.RawMessage{
			json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`),
			json.RawMessage(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
			json.RawMessage(`{"choices":[],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`),
		}, nil, true},
		{"missing terminal", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`)}, nil, false},
		{"failure after terminal", []json.RawMessage{
			json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`),
			json.RawMessage(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
		}, errors.New("backend disconnected"), false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			engine := &llamaRawStreamEngine{MockEngine: NewMockEngine(), chunks: scenario.chunks, failure: scenario.failure}
			if err := engine.Load(context.Background(), "fixture.gguf", LoadOptions{}); err != nil {
				t.Fatal(err)
			}
			request := &api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, Stream: true}
			var events []ProviderEvent
			err := StreamAdmittedLlamaChat(context.Background(), engine, "legacy-active:fixture", 4096, request, func(event ProviderEvent) error {
				events = append(events, event)
				return nil
			})
			if scenario.complete {
				if err != nil || len(events) != 3 || events[0].Type != ProviderEventDelta || events[1].Type != ProviderEventUsage || events[2].Type != ProviderEventDone || !engine.IsLoaded() {
					t.Fatalf("provider bridge lost a completed stream: events=%+v err=%v loaded=%v", events, err, engine.IsLoaded())
				}
			} else {
				if err == nil || len(events) == 0 || events[len(events)-1].Type == ProviderEventDone {
					t.Fatalf("incomplete provider stream was committed: events=%+v err=%v", events, err)
				}
			}
		})
	}
}

func TestAdmittedLlamaChatRejectsToolMenuBeforeGeneration(t *testing.T) {
	engine := &llamaRawStreamEngine{MockEngine: NewMockEngine()}
	request := &api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, Tools: []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "desktop"}}}}
	if err := StreamAdmittedLlamaChat(context.Background(), engine, "legacy-active:fixture", 4096, request, func(ProviderEvent) error { t.Fatal("unqualified tool reached callback"); return nil }); err == nil {
		t.Fatal("tool menu was admitted by the text-only bridge")
	}
}
