//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func nativeToolFixture() *api.ChatCompletionRequest {
	req := singleStepFixture()
	req.SingleStepTools = false
	return req
}

func TestNativeWorkerErrorsAreActionableAndPrivate(t *testing.T) {
	for _, tc := range []struct{ message, code string }{
		{"Failed to parse input at pos 42: PRIVATE", "invalid_model_tool_response"},
		{"device error: PRIVATE", "runtime_stream_error"},
	} {
		raw, _ := json.Marshal(map[string]string{"message": tc.message})
		err := AsEngineError(classifyNativeStreamError(raw))
		if err == nil || err.Code != tc.code || strings.Contains(err.Error(), "PRIVATE") || !strings.Contains(err.Message, "No tool call") {
			t.Fatal(err)
		}
	}
}

func TestNativeDecisionRejectionSurvivesServiceForwarding(t *testing.T) {
	for _, code := range []string{"invalid_model_tool_response", "runtime_stream_error", "inference_timeout"} {
		raw, _ := json.Marshal(map[string]string{"code": code, "message": "PRIVATE"})
		err := classifyNativeStreamError(raw)
		var decision *DecisionValidationError
		if errors.As(err, &decision) != (code == "invalid_model_tool_response") || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("invalid repair classification", code, err)
		}
	}
}

func TestExternalSequentialPreservesNativeWireAndCount(t *testing.T) {
	for _, model := range []string{"qwen", "llama", "phi", "future-renamed-model"} {
		req := nativeToolFixture()
		req.Model = model
		req.Messages = append(req.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "prior", Type: "function", Function: api.FunctionCall{Name: "fixture_read", Arguments: "{}"}}}}, api.ChatMessage{Role: "tool", Name: "fixture_read", ToolCallID: "prior", Content: "test result"})
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var got api.ChatCompletionRequest
			if json.NewDecoder(r.Body).Decode(&got) != nil || got.ResponseFormat != nil || got.SingleStepTools || got.ParallelToolCalls == nil || *got.ParallelToolCalls || !reflect.DeepEqual(got.Messages, req.Messages) || !reflect.DeepEqual(got.Tools, req.Tools) {
				t.Error("public request was rewritten")
			}
			if strings.HasSuffix(r.URL.Path, "/input_tokens") {
				fmt.Fprint(w, `{"input_tokens":123}`)
			} else {
				fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}]}`)
			}
		}))
		engine := NewLlamaHTTPEngine(backend.URL)
		if count, err := engine.CountPromptTokens(context.Background(), req); err != nil || count != 123 {
			t.Fatal(count, err)
		}
		if answer, err := engine.ChatCompletion(context.Background(), req); err != nil || answer.Choices[0].Message.StringContent() != "Hello!" {
			t.Fatal(answer, err)
		}
		backend.Close()
	}
	var public api.ChatCompletionRequest
	if err := json.Unmarshal([]byte(`{"SingleStepTools":true,"single_step_tools":true}`), &public); err != nil || public.SingleStepTools {
		t.Fatal("public JSON selected an internal protocol")
	}
}

func nativeStreamChunk(delta any, finish any) string {
	data, _ := json.Marshal(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
	return "data: " + string(data) + "\n\n"
}

func nativeCallDelta(index int, name, args string) any {
	return map[string]any{"tool_calls": []any{map[string]any{"index": index, "id": fmt.Sprintf("call-%d", index), "type": "function", "function": api.FunctionCall{Name: name, Arguments: args}}}}
}

func TestNativeToolStreamValidatesBeforeReleasingCalls(t *testing.T) {
	valid := nativeStreamChunk(nativeCallDelta(0, "fixture_read", "{}"), nil)
	finish := nativeStreamChunk(map[string]any{}, "tool_calls")
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"valid", valid + finish + "data: [DONE]\n\n", false},
		{"missing done", valid + finish, true},
		{"missing finish", valid + "data: [DONE]\n\n", true},
		{"length", valid + nativeStreamChunk(map[string]any{}, "length") + "data: [DONE]\n\n", true},
		{"unknown", nativeStreamChunk(nativeCallDelta(0, "not_offered", "{}"), nil) + finish + "data: [DONE]\n\n", true},
		{"schema", nativeStreamChunk(nativeCallDelta(0, "fixture_read", `{"extra":1}`), nil) + finish + "data: [DONE]\n\n", true},
		{"duplicate", nativeStreamChunk(nativeCallDelta(0, "fixture_read", `{"a":1,"a":2}`), nil) + finish + "data: [DONE]\n\n", true},
		{"multiple", valid + nativeStreamChunk(nativeCallDelta(1, "fixture_open", "{}"), nil) + finish + "data: [DONE]\n\n", true},
		{"gap", nativeStreamChunk(nativeCallDelta(1, "fixture_read", "{}"), nil) + finish + "data: [DONE]\n\n", true},
		{"error", valid + `data: {"error":{"message":"PRIVATE"}}` + "\n\ndata: [DONE]\n\n", true},
		{"post finish", valid + finish + valid + "data: [DONE]\n\n", true},
		{"malformed", valid + "data: {\n\ndata: [DONE]\n\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer backend.Close()
			var output strings.Builder
			err := NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(context.Background(), nativeToolFixture(), func(raw json.RawMessage) error { output.Write(raw); return nil })
			if (err != nil) != tc.wantError {
				t.Fatal(err)
			}
			if tc.wantError && (strings.Contains(output.String(), "tool_calls") || strings.Contains(output.String(), "fixture_read") || strings.Contains(output.String(), `"finish_reason":"`)) {
				t.Fatalf("invalid call escaped: %s", output.String())
			}
			if !tc.wantError && !strings.Contains(output.String(), `"id":"call-0"`) {
				t.Fatal("native ID lost")
			}
		})
	}
}

func TestNativeToolTextStreamsBeforeTerminalAndPreservesUsage(t *testing.T) {
	first := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nativeStreamChunk(map[string]any{"role": "assistant", "content": "Hello"}, nil))
		w.(http.Flusher).Flush()
		select {
		case <-first:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, nativeStreamChunk(map[string]any{"content": "!"}, "stop"))
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"total_tokens\":12}}\n\ndata: [DONE]\n\n")
	}))
	defer backend.Close()
	var text strings.Builder
	var usage int
	err := NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(context.Background(), nativeToolFixture(), func(raw json.RawMessage) error {
		var chunk api.ChatCompletionChunk
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return err
		}
		for _, choice := range chunk.Choices {
			text.WriteString(choice.Delta.StringContent())
		}
		if text.String() == "Hello" {
			select {
			case <-first:
			default:
				close(first)
			}
		}
		if chunk.Usage != nil {
			usage = chunk.Usage.TotalTokens
		}
		return nil
	})
	if err != nil || text.String() != "Hello!" || usage != 12 {
		t.Fatal(text.String(), usage, err)
	}
}

func TestNativeToolCancellationDoesNotReleaseArguments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nativeStreamChunk(nativeCallDelta(0, "fixture_read", "{}"), nil))
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	}))
	defer backend.Close()
	calls := 0
	err := NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(ctx, nativeToolFixture(), func(json.RawMessage) error { calls++; return nil })
	if err == nil || calls != 0 {
		t.Fatal(calls, err)
	}
}
