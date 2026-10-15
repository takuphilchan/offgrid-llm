//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func singleStepFixture() *api.ChatCompletionRequest {
	parallel := false
	return &api.ChatCompletionRequest{ParallelToolCalls: &parallel, ToolChoice: "auto", Messages: []api.ChatMessage{{Role: "system", Content: "Fixture authority"}, {Role: "user", Content: "Open the fixture and read it"}}, Tools: []api.Tool{
		{Type: "function", Function: api.FunctionDef{Name: "fixture_open", Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}},
		{Type: "function", Function: api.FunctionDef{Name: "fixture_read", Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}},
	}}
}

func TestSingleStepStrictResponseBeforeToolDispatch(t *testing.T) {
	for _, content := range []string{`{"tool_call":{"name":"fixture_open","arguments":{}}}`, `{"message":"Finished — café 日本語"}`, `{"tool_call":{"name":"foreign","arguments":{}}}`, `{"tool_call":{"name":"fixture_open","arguments":[]}}`, `{"tool_call":{"name":"fixture_open","arguments":{}},"message":"done"}`, `{"tool_calls":[]}`, `{"message":"a","message":"b"}`, `{"message":"\ud800"}`} {
		choice := api.ChatCompletionChoice{Message: api.ChatMessage{Content: content}, FinishReason: "stop"}
		err := decodeSingleStep(singleStepFixture(), &choice)
		valid := content == `{"tool_call":{"name":"fixture_open","arguments":{}}}` || content == `{"message":"Finished — café 日本語"}`
		if (err == nil) != valid {
			t.Fatal("invalid envelope acceptance", content, err)
		}
	}
	for _, finish := range []string{"length", "tool_calls", "", "content_filter"} {
		choice := api.ChatCompletionChoice{Message: api.ChatMessage{Content: `{"tool_call":{"name":"fixture_open","arguments":{}}}`}, FinishReason: finish}
		if decodeSingleStep(singleStepFixture(), &choice) == nil {
			t.Fatal("unfinished response accepted", finish)
		}
	}
}

func TestSingleStepCallsHaveDistinctCorrelationIdentities(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		choice := api.ChatCompletionChoice{Message: api.ChatMessage{Content: `{"tool_call":{"name":"fixture_open","arguments":{}}}`}, FinishReason: "stop"}
		if err := decodeSingleStep(singleStepFixture(), &choice); err != nil {
			t.Fatal(err)
		}
		id := choice.Message.ToolCalls[0].ID
		if !strings.HasPrefix(id, "call_") || len(id) != 37 || seen[id] {
			t.Fatal("missing or reused tool-result correlation identity")
		}
		seen[id] = true
	}
}

func TestSingleStepEmbedsToolLocalReferencesWithoutCollisions(t *testing.T) {
	request := singleStepFixture()
	for i := range request.Tools {
		request.Tools[i].Function.Parameters = map[string]any{"type": "object", "$defs": map[string]any{"item": map[string]any{"type": "string", "const": fmt.Sprint(i)}}, "properties": map[string]any{"value": map[string]any{"$ref": "#/$defs/item"}}}
	}
	body, err := marshalLlamaRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	for i := range request.Tools {
		want := fmt.Sprintf("#/oneOf/%d/properties/tool_call/properties/arguments/$defs/item", i)
		if !strings.Contains(string(body), want) {
			t.Fatal("tool-local reference lost", want)
		}
		p := request.Tools[i].Function.Parameters["properties"].(map[string]any)["value"].(map[string]any)
		if p["$ref"] != "#/$defs/item" {
			t.Fatal("canonical schema identity changed")
		}
	}
}

func TestSingleStepPreservesForcedToolAndImages(t *testing.T) {
	req := singleStepFixture()
	req.ToolChoice = map[string]any{"type": "function", "function": map[string]any{"name": "fixture_read"}}
	req.Messages = append(req.Messages, api.ChatMessage{Role: "user", Content: []api.ChatContentPart{{Type: "text", Text: "Owned image observation"}, {Type: "image_url", ImageURL: &api.ChatImageURL{URL: "data:image/png;base64,fixture"}}}})
	body, err := marshalLlamaRequest(req)
	if err != nil || !strings.Contains(string(body), `"image_url"`) {
		t.Fatal("image or forced choice lost", err)
	}
	choice := api.ChatCompletionChoice{Message: api.ChatMessage{Content: `{"tool_call":{"name":"fixture_open","arguments":{}}}`}, FinishReason: "stop"}
	if decodeSingleStep(req, &choice) == nil {
		t.Fatal("selected tool widened")
	}
	choice.Message.Content = `{"message":"done"}`
	if decodeSingleStep(req, &choice) == nil {
		t.Fatal("forced action replaced by text")
	}
}

func TestSingleStepChangingCatalogPreservesStablePrefixAndCanonicalHistory(t *testing.T) {
	request := singleStepFixture()
	request.Messages = append(request.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "fixture_open", Arguments: "{}"}}}}, api.ChatMessage{Role: "tool", Name: "fixture_open", Content: "Owned result"})
	decode := func() []api.ChatMessage {
		body, err := marshalLlamaRequest(request)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Messages []api.ChatMessage `json:"messages"`
		}
		if json.Unmarshal(body, &wire) != nil {
			t.Fatal("bad wire")
		}
		return wire.Messages
	}
	first := decode()
	request.Tools = request.Tools[1:]
	second := decode()
	if first[0].StringContent() != second[0].StringContent() || strings.Contains(first[0].StringContent(), "fixture_open") {
		t.Fatal("catalog invalidated stable system prefix")
	}
	if first[len(first)-1].StringContent() != second[len(second)-1].StringContent() || request.Messages[len(request.Messages)-1].Role != "tool" || len(request.Messages[2].ToolCalls) != 1 {
		t.Fatal("lost final observation or mutated canonical history")
	}
}

func TestSingleStepHTTPCountStreamAndCompletionShareWireContract(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			req := singleStepFixture()
			var counted map[string]json.RawMessage
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire map[string]json.RawMessage
				if json.NewDecoder(r.Body).Decode(&wire) != nil || wire["tools"] != nil || wire["response_format"] == nil {
					t.Error("native parser still controls sequential calls")
				}
				if r.URL.Path == "/v1/chat/completions/input_tokens" {
					counted = wire
					fmt.Fprint(w, `{"input_tokens":321}`)
					return
				}
				for _, field := range []string{"messages", "response_format"} {
					if string(counted[field]) != string(wire[field]) {
						t.Error("token count diverged from inference", field)
					}
				}
				if stream {
					for _, fragment := range []string{`{"tool_call":`, `{"name":"fixture_open","arguments":{}}}`} {
						b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": fragment}}}})
						fmt.Fprintf(w, "data: %s\n\n", b)
					}
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":321}}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_call\":{\"name\":\"fixture_open\",\"arguments\":{}}}"},"finish_reason":"stop"}]}`)
				}
			}))
			defer backend.Close()
			engine := NewLlamaHTTPEngine(backend.URL)
			if count, err := engine.CountPromptTokens(context.Background(), req); err != nil || count != 321 {
				t.Fatal(count, err)
			}
			if stream {
				callbacks := 0
				err := engine.ChatCompletionStreamRaw(context.Background(), req, func(data json.RawMessage) error {
					callbacks++
					if !strings.Contains(string(data), `"finish_reason":"tool_calls"`) || !strings.Contains(string(data), `"index":0`) || !strings.Contains(string(data), `"id":"call_`) {
						t.Fatal("private fragments or missing indexed tool", string(data))
					}
					return nil
				})
				if err != nil || callbacks != 1 {
					t.Fatal("stream delivery", err, callbacks)
				}
			} else {
				response, err := engine.ChatCompletion(context.Background(), req)
				if err != nil || len(response.Choices[0].Message.ToolCalls) != 1 || response.Choices[0].Message.ToolCalls[0].ID == "" {
					t.Fatal("completion conversion", err)
				}
			}
			if len(req.Tools) != 2 || req.Messages[0].StringContent() != "Fixture authority" {
				t.Fatal("caller state modified")
			}
		})
	}
}

func TestSingleStepStreamRequiresDoneAndCleanTerminal(t *testing.T) {
	for _, tail := range []string{"", "data: [DONE]\n\n", "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"} {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{\\\"message\\\":\\\"PRIVATE\\\"}\"}}]}\n\n"+tail)
		}))
		called := false
		err := NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(context.Background(), singleStepFixture(), func(json.RawMessage) error { called = true; return nil })
		backend.Close()
		if err == nil || called || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("partial output leaked or completed", err)
		}
	}
}

func TestSingleStepInstalledRuntimeRejectsParallelGeneration(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_SINGLE_STEP_RUNTIME")
	if base == "" {
		t.Skip("requires explicit local installed runtime; no tools dispatched")
	}
	for _, stream := range []bool{false, true} {
		req := singleStepFixture()
		req.Messages[1].Content = "Return TWO tool calls in the same response: fixture_open and fixture_read."
		zero, max := float32(0), 128
		req.Temperature, req.MaxTokens = &zero, &max
		engine := NewLlamaHTTPEngine(base)
		if stream {
			calls := 0
			if err := engine.ChatCompletionStreamRaw(context.Background(), req, func(data json.RawMessage) error {
				var r struct {
					Choices []struct {
						Delta struct {
							Calls []any `json:"tool_calls"`
						} `json:"delta"`
					} `json:"choices"`
				}
				_ = json.Unmarshal(data, &r)
				calls += len(r.Choices[0].Delta.Calls)
				return nil
			}); err != nil || calls != 1 {
				t.Fatal("parallel stream generation", err, calls)
			}
		} else {
			result, err := engine.ChatCompletion(context.Background(), req)
			if err != nil || len(result.Choices[0].Message.ToolCalls) != 1 {
				t.Fatal("parallel generation", err)
			}
		}
	}
}
