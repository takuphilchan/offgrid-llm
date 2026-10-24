//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLlamaForwardsStructuredResponseConstraint(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var raw map[string]json.RawMessage
				if json.NewDecoder(r.Body).Decode(&raw) != nil || raw["response_format"] == nil {
					t.Error("constraint dropped")
				}
				var format api.ResponseFormat
				if json.Unmarshal(raw["response_format"], &format) != nil || format.JSONSchema == nil || format.JSONSchema.Schema["type"] != "object" {
					t.Error("schema lost")
				}
				if stream {
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{}\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
				}
			}))
			defer server.Close()
			engine := NewLlamaHTTPEngine(server.URL)
			req := &api.ChatCompletionRequest{Model: "fixture", Messages: []api.ChatMessage{{Role: "user", Content: "Return JSON"}}, ResponseFormat: &api.ResponseFormat{Type: "json_schema", JSONSchema: &api.ResponseJSONSchema{Name: "title", Schema: map[string]any{"type": "object"}}}}
			var err error
			if stream {
				err = engine.ChatCompletionStream(context.Background(), req, func(string) error { return nil })
			} else {
				_, err = engine.ChatCompletion(context.Background(), req)
			}
			if err != nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestNativeTextRejectsUnqualifiedResponseFormat(t *testing.T) {
	_, err := nativeTextPrompt(&api.ChatCompletionRequest{Model: "native", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}, ResponseFormat: &api.ResponseFormat{Type: "json_object"}}, ProviderDescriptor{})
	if err == nil {
		t.Fatal("native provider silently ignored constraint")
	}
}
