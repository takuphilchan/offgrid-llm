package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type structuredStreamTestEngine struct {
	inference.Engine
}

type interruptedStreamTestEngine struct{ inference.Engine }

func (e *interruptedStreamTestEngine) ChatCompletionStreamRaw(_ context.Context, _ *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	if err := callback(json.RawMessage(`{"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}`)); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func TestPublicStreamDoesNotConvertPartialEOFToSuccess(t *testing.T) {
	s := &Server{engine: &interruptedStreamTestEngine{}}
	w := httptest.NewRecorder()
	s.handleChatCompletionsStream(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), &api.ChatCompletionRequest{Model: "test"})
	body := w.Body.String()
	if !strings.Contains(body, `"error"`) || strings.Contains(body, "[DONE]") || strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("partial output reported as success: %s", body)
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

	server.handleChatCompletionsStream(recorder, request, &api.ChatCompletionRequest{Model: "agent-model", Stream: true})
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
