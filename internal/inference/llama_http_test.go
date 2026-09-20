package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestLlamaHTTPPortUsesExplicitIPv4Loopback(t *testing.T) {
	engine := NewLlamaHTTPEngine("")
	if engine.baseURL != "http://127.0.0.1:42382" {
		t.Fatalf("default base URL = %q", engine.baseURL)
	}
	engine.SetPort(43123)
	if engine.baseURL != "http://127.0.0.1:43123" {
		t.Fatalf("base URL after SetPort = %q", engine.baseURL)
	}
}

func TestHTTPRequestsPreserveParallelToolPolicy(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, mode := range []string{"unset", "enabled", "disabled"} {
			t.Run(fmt.Sprintf("stream=%v/%s", stream, mode), func(t *testing.T) {
				var parallel *bool
				if mode != "unset" {
					value := mode == "enabled"
					parallel = &value
				}
				backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					value, exists := body["parallel_tool_calls"]
					if parallel == nil {
						if exists {
							t.Error("unset policy was sent")
						}
					} else if !exists || string(value) != fmt.Sprint(*parallel) {
						t.Errorf("policy lost: %s", value)
					}
					if stream {
						fmt.Fprint(w, "data: [DONE]\n\n")
					} else {
						fmt.Fprint(w, `{"choices":[]}`)
					}
				}))
				defer backend.Close()
				engine := NewLlamaHTTPEngine(backend.URL)
				request := &api.ChatCompletionRequest{ParallelToolCalls: parallel}
				var err error
				if stream {
					err = engine.ChatCompletionStreamRaw(context.Background(), request, func(json.RawMessage) error { return nil })
				} else {
					_, err = engine.ChatCompletion(context.Background(), request)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestHTTPRequestsPreserveSeedAndOnlyExplicitStops(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, stops := range [][]string{nil, {"CUSTOM_END"}} {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request api.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request.Seed == nil || *request.Seed != 0 || len(request.Stop) != len(stops) {
					t.Errorf("changed sampling or stops: %+v", request)
				}
				if len(stops) > 0 && request.Stop[0] != stops[0] {
					t.Error("explicit stop changed")
				}
				if stream {
					fmt.Fprint(w, "data: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[]}`)
				}
			}))
			seed := int64(0)
			request := &api.ChatCompletionRequest{Model: "phi-3-renamed", Seed: &seed, Stop: stops}
			engine := NewLlamaHTTPEngine(backend.URL)
			var err error
			if stream {
				err = engine.ChatCompletionStreamRaw(context.Background(), request, func(json.RawMessage) error { return nil })
			} else {
				_, err = engine.ChatCompletion(context.Background(), request)
			}
			backend.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestRawStreamRequiresTerminalMarkerAndPropagatesCallbackFailure(t *testing.T) {
	chunk := "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"
	for _, test := range []struct {
		name, body   string
		failCallback bool
		want         error
	}{
		{"complete", chunk + "data: [DONE]\n\n", false, nil},
		{"empty EOF", "", false, io.ErrUnexpectedEOF},
		{"partial EOF", chunk, false, io.ErrUnexpectedEOF},
		{"callback after partial", chunk + chunk + "data: [DONE]\n\n", true, io.ErrClosedPipe},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, test.body)
			}))
			defer backend.Close()
			calls := 0
			err := NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(context.Background(), &api.ChatCompletionRequest{}, func(json.RawMessage) error {
				calls++
				if test.failCallback && calls == 2 {
					return io.ErrClosedPipe
				}
				return nil
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestRawStreamCancellationIsNotSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewLlamaHTTPEngine("http://127.0.0.1:1").ChatCompletionStreamRaw(ctx, &api.ChatCompletionRequest{}, func(json.RawMessage) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestContextOverflowGivesActionableProfileError(t *testing.T) {
	err := classifyLlamaServerError(400, []byte(`{"error":{"message":"request exceeds the available context size"}}`))
	typed := AsEngineError(err)
	if typed == nil || typed.Code != "context_exceeded" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLlamaHTTPRawStreamPreservesToolCallsAndUsage(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var request api.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.StreamOptions == nil || !request.StreamOptions.IncludeUsage {
			t.Fatal("stream_options.include_usage was not forwarded")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"backend\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"weather\",\"arguments\":\"{\\\"city\\\":\\\"Nairobi\\\"}\"}}]},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"id\":\"backend\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":4,\"total_tokens\":14}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer backend.Close()

	engine := NewLlamaHTTPEngine(backend.URL)
	request := &api.ChatCompletionRequest{
		Model:         "agent-model",
		Messages:      []api.ChatMessage{{Role: "user", Content: "weather"}},
		StreamOptions: &api.StreamOptions{IncludeUsage: true},
	}
	var chunks []map[string]interface{}
	err := engine.ChatCompletionStreamRaw(context.Background(), request, func(data json.RawMessage) error {
		var chunk map[string]interface{}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return err
		}
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	choices := chunks[0]["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if _, ok := delta["tool_calls"]; !ok {
		t.Fatalf("tool_calls missing from %#v", chunks[0])
	}
	usage := chunks[1]["usage"].(map[string]interface{})
	if usage["total_tokens"] != float64(14) {
		t.Fatalf("usage missing from %#v", chunks[1])
	}
}
