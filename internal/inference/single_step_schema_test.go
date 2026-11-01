//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSingleStepPatternProjectionPreservesOriginalValidation(t *testing.T) {
	const pattern = `^[^/\\:]+\.txt$`
	for _, stream := range []bool{false, true} {
		for _, name := range []string{"draft.txt", "café 日本語.txt", `quoted"name.txt`, "../escape.txt", "wrong.md"} {
			t.Run(fmt.Sprintf("stream=%t/%s", stream, name), func(t *testing.T) {
				req := singleStepFixture()
				req.Tools = []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "save_fixture", Parameters: map[string]any{
					"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string", "pattern": pattern, "minLength": 1, "maxLength": 120}},
					"required": []string{"name"}, "additionalProperties": false,
				}}}}
				before, _ := json.Marshal(req)
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var sent struct {
						Messages []api.ChatMessage `json:"messages"`
						Format   struct {
							Schema struct {
								Value map[string]any `json:"schema"`
							} `json:"json_schema"`
						} `json:"response_format"`
					}
					if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
						t.Error(err)
						return
					}
					branch := sent.Format.Schema.Value["oneOf"].([]any)[0].(map[string]any)
					call := branch["properties"].(map[string]any)["tool_call"].(map[string]any)
					args := call["properties"].(map[string]any)["arguments"].(map[string]any)
					field := args["properties"].(map[string]any)["name"].(map[string]any)
					if _, exists := field["pattern"]; exists || field["type"] != "string" || field["maxLength"] != float64(120) || field["minLength"] != float64(1) {
						t.Errorf("unsafe generation schema: %v", field)
					}
					catalog, _ := json.Marshal(req.Tools)
					found := false
					for _, message := range sent.Messages {
						found = found || strings.Contains(message.StringContent(), string(catalog))
					}
					if !found {
						t.Error("original schema missing from model catalog")
					}
					argsJSON, _ := json.Marshal(map[string]string{"name": name})
					proposal := `{"tool_call":{"name":"save_fixture","arguments":` + string(argsJSON) + `}}`
					if stream {
						chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": proposal}, "finish_reason": "stop"}}})
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
					} else {
						json.NewEncoder(w).Encode(api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: proposal}}}})
					}
				}))
				defer backend.Close()
				engine := NewLlamaHTTPEngine(backend.URL)
				var err error
				emitted := false
				if stream {
					err = engine.ChatCompletionStreamRaw(context.Background(), req, func(json.RawMessage) error { emitted = true; return nil })
				} else {
					_, err = engine.ChatCompletion(context.Background(), req)
				}
				valid := name != "../escape.txt" && name != "wrong.md"
				if (err == nil) != valid {
					t.Fatalf("original pattern enforcement: valid=%t err=%v", valid, err)
				}
				if stream && emitted != valid {
					t.Fatal("invalid decision reached public stream or valid decision was lost")
				}
				after, _ := json.Marshal(req)
				if string(before) != string(after) {
					t.Fatal("generation projection changed caller schema")
				}
			})
		}
	}
}

func TestSingleStepPatternProjectionOnlyVisitsSchemas(t *testing.T) {
	const raw = `{"type":"object","$defs":{"name":{"type":"string","pattern":"^.*$"}},"properties":{"nested":{"type":"array","items":{"oneOf":[{"type":"string","pattern":"^.*$"}]}},"pattern":{"const":{"pattern":"literal data"}},"reference":{"$ref":"#/$defs/name"}},"enum":[{"pattern":"literal data"}],"default":{"pattern":"literal data"}}`
	var original map[string]any
	if err := json.Unmarshal([]byte(raw), &original); err != nil {
		t.Fatal(err)
	}
	projected, err := embedSingleStepParameters(original, "#/fixture")
	if err != nil {
		t.Fatal(err)
	}
	defs := projected["$defs"].(map[string]any)["name"].(map[string]any)
	if _, exists := defs["pattern"]; exists {
		t.Fatal("definition pattern reached grammar")
	}
	props := projected["properties"].(map[string]any)
	items := props["nested"].(map[string]any)["items"].(map[string]any)
	if _, exists := items["oneOf"].([]any)[0].(map[string]any)["pattern"]; exists {
		t.Fatal("nested pattern reached grammar")
	}
	for _, key := range []string{"enum", "default"} {
		if !reflect.DeepEqual(projected[key], original[key]) {
			t.Fatalf("changed data keyword %s", key)
		}
	}
	if !reflect.DeepEqual(props["pattern"], original["properties"].(map[string]any)["pattern"]) {
		t.Fatal("changed const data")
	}
	if props["reference"].(map[string]any)["$ref"] != "#/fixture/$defs/name" {
		t.Fatal("lost local reference rebasing")
	}
}

func TestSingleStepToolContinuationKeepsCatalogPrefix(t *testing.T) {
	request := singleStepFixture()
	request.Messages = append([]api.ChatMessage{
		{Role: "system", Content: "Stable authority"},
		{Role: "user", Content: "Earlier conversation"},
		{Role: "assistant", Content: "Earlier answer"},
	}, request.Messages[1:]...)
	wire := func() []api.ChatMessage {
		t.Helper()
		body, err := marshalLlamaRequest(request)
		if err != nil {
			t.Fatal(err)
		}
		var sent api.ChatCompletionRequest
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatal(err)
		}
		return sent.Messages
	}
	first := wire()
	request.Messages = append(request.Messages,
		api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "saved-call", Type: "function", Function: api.FunctionCall{Name: "fixture_open", Arguments: "{}"}}}},
		api.ChatMessage{Role: "tool", Name: "fixture_open", ToolCallID: "saved-call", Content: "Observed result"},
	)
	next := wire()
	if len(next) != len(first)+2 || !reflect.DeepEqual(first, next[:len(first)]) {
		t.Fatal("unchanged catalog moved within the current exchange, invalidating prompt cache")
	}
	if !strings.Contains(next[len(next)-1].StringContent(), "Observed result") || request.Messages[len(request.Messages)-1].Role != "tool" {
		t.Fatal("tool result lost or canonical history mutated")
	}
}
