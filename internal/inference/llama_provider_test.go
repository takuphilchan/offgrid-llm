package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestNewLlamaCPPProviderRequiresPinnedIdentity(t *testing.T) {
	_, err := NewLlamaCPPProvider("http://127.0.0.1:42382", ProviderDescriptor{ProtocolRevision: "provider-v1", RunnerRevision: "fixture-1", ModelDigest: "sha256:test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewLlamaCPPProvider("", ProviderDescriptor{Provider: ProviderLlamaCPP, Format: FormatGGUF})
	if err == nil {
		t.Fatal("provider without immutable identity accepted")
	}
}

func llamaProviderDescriptor(capabilities ...ProviderCapability) ProviderDescriptor {
	return ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderLlamaCPP, Format: FormatGGUF, RunnerRevision: "fixture-1", ModelDigest: "sha256:test", Capabilities: capabilities}
}

func TestLlamaProviderStreamsTextThroughExistingEngine(t *testing.T) {
	engine := NewMockEngine()
	if err := engine.Load(context.Background(), "fixture.gguf", LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityStreaming))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}})
	req := ProviderRequest{RequestID: "one", CancellationID: "cancel-one", ModelDigest: "sha256:test", Prompt: prompt}
	var sequence uint64
	var deltas int
	err = provider.Generate(context.Background(), req, func(event ProviderEvent) error {
		if event.Sequence != sequence+1 || event.RequestID != req.RequestID {
			t.Fatalf("event identity/sequence changed: %#v", event)
		}
		sequence = event.Sequence
		if event.Type == ProviderEventDelta {
			deltas++
		}
		if event.Type == ProviderEventDone && event.FinishReason != "stop" {
			t.Fatalf("incomplete terminal event: %#v", event)
		}
		return nil
	})
	if err != nil || deltas == 0 || sequence != uint64(deltas+1) {
		t.Fatalf("text stream failed: deltas=%d sequence=%d err=%v", deltas, sequence, err)
	}
	if err := provider.Generate(context.Background(), req, func(ProviderEvent) error { return nil }); err == nil {
		t.Fatal("duplicate llama request identity was accepted")
	}
	if err := provider.Close(); err != nil || !engine.IsLoaded() {
		t.Fatalf("closing adapter unloaded service-owned engine: %v", err)
	}
}

type llamaDecisionEngine struct {
	*MockEngine
	response *api.ChatCompletionResponse
}

func (e *llamaDecisionEngine) ChatCompletion(context.Context, *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	return e.response, nil
}

func TestLlamaProviderRejectsInvalidToolBeforeEmission(t *testing.T) {
	engine := &llamaDecisionEngine{MockEngine: NewMockEngine(), response: &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: "unknown", Arguments: `{}`}}}}, FinishReason: "tool_calls"}}}}
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityToolCalls))
	if err != nil {
		t.Fatal(err)
	}
	chat := api.ChatCompletionRequest{Model: "fixture", Tools: []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "allowed", Parameters: map[string]any{"type": "object"}}}}}
	prompt, _ := json.Marshal(chat)
	req := ProviderRequest{RequestID: "one", CancellationID: "cancel-one", ModelDigest: "sha256:test", Prompt: prompt}
	called := false
	if err := provider.Generate(context.Background(), req, func(ProviderEvent) error { called = true; return nil }); err == nil || called {
		t.Fatalf("invalid tool was emitted: called=%v err=%v", called, err)
	}
}

func TestLlamaProviderRejectsUndeclaredToolCatalogBeforeGeneration(t *testing.T) {
	engine := &llamaDecisionEngine{MockEngine: NewMockEngine(), response: &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Index: 0, Message: api.ChatMessage{Role: "assistant", Content: "answer"}, FinishReason: "stop"}}}}
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityToolCalls))
	if err != nil {
		t.Fatal(err)
	}
	chat := api.ChatCompletionRequest{Model: "fixture", Tools: []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "unexpected", Parameters: map[string]any{"type": "object"}}}}}
	prompt, _ := json.Marshal(chat)
	request := ProviderRequest{RequestID: "undeclared", CancellationID: "undeclared", ModelDigest: "sha256:test", Prompt: prompt}
	if err := provider.Generate(context.Background(), request, func(ProviderEvent) error { t.Fatal("undeclared tool reached emission"); return nil }); err == nil {
		t.Fatal("prompt-supplied tool catalog was accepted without a declared menu")
	}
}

type llamaBlockingEngine struct {
	*MockEngine
	started chan struct{}
}

type llamaRawStreamEngine struct {
	*MockEngine
	chunks       []json.RawMessage
	failure      error
	includeUsage bool
}

func (e *llamaRawStreamEngine) ChatCompletionStreamRaw(_ context.Context, req *api.ChatCompletionRequest, callback ChatCompletionStreamCallback) error {
	e.includeUsage = req.StreamOptions != nil && req.StreamOptions.IncludeUsage
	for _, chunk := range e.chunks {
		if err := callback(chunk); err != nil {
			return err
		}
	}
	return e.failure
}

func TestLlamaProviderPreservesRawStreamFinishAndUsage(t *testing.T) {
	engine := &llamaRawStreamEngine{MockEngine: NewMockEngine(), chunks: []json.RawMessage{
		json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"Hello","tool_calls":[]},"finish_reason":null}]}`),
		json.RawMessage(`{"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`),
		json.RawMessage(`{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`),
	}}
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityStreaming))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}})
	request := ProviderRequest{RequestID: "raw-1", CancellationID: "raw-cancel-1", ModelDigest: "sha256:test", Prompt: prompt}
	var events []ProviderEvent
	if err := provider.Generate(context.Background(), request, func(event ProviderEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	if !engine.includeUsage || len(events) != 3 || events[0].Type != ProviderEventDelta || events[0].Text != "Hello" || events[1].Type != ProviderEventUsage || events[1].Usage.TotalTokens != 4 || events[2].Type != ProviderEventDone || events[2].FinishReason != "length" {
		t.Fatalf("raw llama stream lost contract metadata: %+v", events)
	}
}

func TestLlamaProviderPreservesMetadataFromHTTPStream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var request api.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.Stream || request.StreamOptions == nil || !request.StreamOptions.IncludeUsage {
			http.Error(w, "missing usage stream request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer backend.Close()
	engine := NewLlamaHTTPEngine(backend.URL)
	engine.loaded = true
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityStreaming))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}})
	request := ProviderRequest{RequestID: "http-1", CancellationID: "http-1", ModelDigest: "sha256:test", Prompt: prompt}
	var events []ProviderEvent
	if err := provider.Generate(context.Background(), request, func(event ProviderEvent) error { events = append(events, event); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Text != "Hello" || events[1].Type != ProviderEventUsage || events[1].Usage.TotalTokens != 4 || events[2].FinishReason != "length" {
		t.Fatalf("HTTP llama stream metadata was lost: %+v", events)
	}
}

func TestLlamaProviderRejectsIncompleteOrUnexpectedRawText(t *testing.T) {
	for _, test := range []struct {
		name    string
		chunks  []json.RawMessage
		failure error
	}{
		{"missing finish", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"partial"}}]}`)}, nil},
		{"transport failure", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`)}, errors.New("connection closed")},
		{"tool delta", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"unsafe"}]},"finish_reason":null}]}`)}, nil},
		{"duplicate usage", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"text"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), json.RawMessage(`{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)}, nil},
		{"duplicate keys", []json.RawMessage{json.RawMessage(`{"choices":[{"index":0,"delta":{"content":"text"},"finish_reason":"stop"}],"choices":[]}`)}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := &llamaRawStreamEngine{MockEngine: NewMockEngine(), chunks: test.chunks, failure: test.failure}
			provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText, CapabilityStreaming))
			if err != nil {
				t.Fatal(err)
			}
			prompt, _ := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}})
			request := ProviderRequest{RequestID: "raw-2", CancellationID: "raw-cancel-2", ModelDigest: "sha256:test", Prompt: prompt}
			var done bool
			if err := provider.Generate(context.Background(), request, func(event ProviderEvent) error { done = done || event.Type == ProviderEventDone; return nil }); err == nil || done {
				t.Fatalf("invalid raw stream became a completed answer: done=%v err=%v", done, err)
			}
		})
	}
}

func (e *llamaBlockingEngine) ChatCompletionStream(ctx context.Context, _ *api.ChatCompletionRequest, _ TokenCallback) error {
	close(e.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestLlamaProviderCancelTargetsActiveGeneration(t *testing.T) {
	engine := &llamaBlockingEngine{MockEngine: NewMockEngine(), started: make(chan struct{})}
	provider, err := NewLlamaCPPProviderForEngine(engine, llamaProviderDescriptor(CapabilityText))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := json.Marshal(api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}})
	req := ProviderRequest{RequestID: "one", CancellationID: "cancel-one", ModelDigest: "sha256:test", Prompt: prompt}
	done := make(chan error, 1)
	go func() { done <- provider.Generate(context.Background(), req, func(ProviderEvent) error { return nil }) }()
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("generation did not begin")
	}
	if err := provider.Cancel(context.Background(), "unrelated"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		t.Fatal("unrelated cancellation stopped the request")
	case <-time.After(20 * time.Millisecond):
	}
	if err := provider.Cancel(context.Background(), req.CancellationID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected request cancellation, got %v", err)
	}
}
