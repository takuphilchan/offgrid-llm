package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type structuredStreamTestEngine struct {
	inference.Engine
}

type scriptedRawStreamEngine struct {
	inference.Engine
	chunks []json.RawMessage
}

func (e *scriptedRawStreamEngine) ChatCompletionStreamRaw(_ context.Context, _ *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	for _, chunk := range e.chunks {
		if err := callback(chunk); err != nil {
			return err
		}
	}
	return nil
}

type interruptedStreamTestEngine struct{ inference.Engine }

type beforeOutputFailureEngine struct {
	inference.Engine
	calls int
}

func (e *beforeOutputFailureEngine) ChatCompletionStreamRaw(context.Context, *api.ChatCompletionRequest, inference.ChatCompletionStreamCallback) error {
	e.calls++
	return io.ErrUnexpectedEOF
}

type retryDetectLlamaProcess struct{ activations int }

func testStreamLease(t *testing.T) *inference.LegacyGenerationLease {
	t.Helper()
	lease, err := inference.NewLegacyGenerationLease(func() {}, inference.ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Release)
	return lease
}

func (p *retryDetectLlamaProcess) Activate(context.Context, string, string, string, inference.LoadOptions) (int, error) {
	p.activations++
	return 12345, nil
}
func (p *retryDetectLlamaProcess) Alive(string) bool                       { return true }
func (p *retryDetectLlamaProcess) Generation(string) uint64                { return 1 }
func (p *retryDetectLlamaProcess) RetireAll(context.Context) (bool, error) { return false, nil }

func (e *interruptedStreamTestEngine) ChatCompletionStreamRaw(_ context.Context, _ *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	if err := callback(json.RawMessage(`{"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}`)); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func TestPublicStreamDoesNotConvertPartialEOFToSuccess(t *testing.T) {
	s := &Server{engine: &interruptedStreamTestEngine{}}
	w := httptest.NewRecorder()
	s.handleChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), &api.ChatCompletionRequest{Model: "test"}, testStreamLease(t))
	body := w.Body.String()
	if !strings.Contains(body, `"error"`) || strings.Contains(body, "[DONE]") || strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("partial output reported as success: %s", body)
	}
}

func TestPublicStreamRejectsMissingAdmission(t *testing.T) {
	s := &Server{engine: &interruptedStreamTestEngine{}}
	w := httptest.NewRecorder()
	s.handleChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), &api.ChatCompletionRequest{Model: "test"}, nil)
	if w.Code != 503 || strings.Contains(w.Body.String(), "data:") {
		t.Fatalf("unadmitted stream was dispatched: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPublicStreamNeverReloadsAndRetriesAfterAmbiguousEOF(t *testing.T) {
	s := newTestServer(t)
	modelPath := filepath.Join(s.config.ModelsDir, "retry-fixture.gguf")
	if err := os.WriteFile(modelPath, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	models := s.registry.ListModels()
	if len(models) != 1 {
		t.Fatalf("expected one isolated fixture model, got %d", len(models))
	}
	engine := &beforeOutputFailureEngine{Engine: inference.NewMockEngine()}
	process := &retryDetectLlamaProcess{}
	s.engine = engine
	s.llamaProcess = process
	// Exercise the old compatibility branch as well as the normal scheduler
	// path. A missing lifecycle gate must not authorize a blind process reload.
	s.inferenceLifecycle = nil
	w := httptest.NewRecorder()
	s.handleChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), &api.ChatCompletionRequest{Model: models[0].ID, Stream: true}, testStreamLease(t))
	if engine.calls != 1 || process.activations != 0 || !strings.Contains(w.Body.String(), `"error"`) || strings.Contains(w.Body.String(), "[DONE]") {
		t.Fatalf("ambiguous generation was retried or reported complete: calls=%d activations=%d body=%s", engine.calls, process.activations, w.Body.String())
	}
}

func TestPublicStreamRequiresOneTerminalChoice(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chunks []json.RawMessage
	}{
		{name: "no terminal", chunks: []json.RawMessage{json.RawMessage(`{"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}`)}},
		{name: "continued after terminal", chunks: []json.RawMessage{
			json.RawMessage(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`),
			json.RawMessage(`{"choices":[{"delta":{"content":"late"},"finish_reason":null}]}`),
		}},
		{name: "duplicate terminal", chunks: []json.RawMessage{
			json.RawMessage(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`),
			json.RawMessage(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{engine: &scriptedRawStreamEngine{chunks: tc.chunks}}
			w := httptest.NewRecorder()
			s.handleChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), &api.ChatCompletionRequest{Model: "fixture", Stream: true}, testStreamLease(t))
			if body := w.Body.String(); !strings.Contains(body, `"error"`) || strings.Contains(body, "[DONE]") {
				t.Fatalf("incomplete stream was reported successful: %s", body)
			}
		})
	}
}

func (e *structuredStreamTestEngine) ChatCompletionStreamRaw(_ context.Context, _ *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	chunks := []string{
		`{"id":"backend","object":"chat.completion.chunk","model":"backend-model","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"Nairobi\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"id":"backend","object":"chat.completion.chunk","model":"backend-model","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
	}
	for _, chunk := range chunks {
		if err := callback(json.RawMessage(chunk)); err != nil {
			return err
		}
	}
	return nil
}

func TestPublicStreamPreservesStructuredToolCalls(t *testing.T) {
	server := newTestServer(t)
	server.engine = &structuredStreamTestEngine{Engine: server.engine}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	server.handleChatCompletionsStream(recorder, request, &api.ChatCompletionRequest{Model: "agent-model", Stream: true}, testStreamLease(t))
	body := recorder.Body.String()
	for _, expected := range []string{`"tool_calls"`, `"name":"weather"`, `"model":"agent-model"`, `"usage"`, "data: [DONE]"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("stream missing %q:\n%s", expected, body)
		}
	}
	if strings.Count(body, "data: [DONE]") != 1 {
		t.Fatalf("stream must contain one terminator:\n%s", body)
	}
}
