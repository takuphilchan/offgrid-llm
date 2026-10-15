//go:build !llama

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Exercise the real private-envelope -> public SSE -> runner parser boundary.
// A mocked Engine returning its own ID cannot catch a missing adapter ID.
func TestNativePreflightThroughSingleStepTransport(t *testing.T) {
	for _, forged := range []bool{false, true} {
		t.Run(fmt.Sprint(forged), func(t *testing.T) {
			calls := 0
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire struct {
					Messages []api.ChatMessage `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&wire) != nil {
					t.Error("invalid wire")
					return
				}
				calls++
				name, args := "computer_observe", map[string]string{}
				if calls == 2 {
					var wrapper struct {
						Result struct {
							Name    string `json:"name"`
							Content string `json:"content"`
						} `json:"tool_result"`
					}
					if json.Unmarshal([]byte(wire.Messages[len(wire.Messages)-1].StringContent()), &wrapper) != nil || wrapper.Result.Name != "computer_observe" {
						t.Error("missing result identity")
						return
					}
					var view struct {
						Requested   string `json:"requested_text"`
						Observation struct {
							ID string `json:"id"`
						} `json:"observation"`
						Elements []struct {
							ID string `json:"id"`
						} `json:"elements"`
					}
					if json.Unmarshal([]byte(wrapper.Result.Content), &view) != nil || len(view.Elements) != 1 {
						t.Error("invalid synthetic observation")
						return
					}
					name = "computer_replace_text"
					args = map[string]string{"observation_id": view.Observation.ID, "element": view.Elements[0].ID, "text": view.Requested}
					if forged {
						args["element"] = "invented"
					}
				}
				text, _ := json.Marshal(map[string]any{"tool_call": map[string]any{"name": name, "arguments": args}})
				chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": string(text)}, "finish_reason": "stop"}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			}))
			defer backend.Close()
			err := probeComputerToolsDriver(context.Background(), inference.NewLlamaHTTPEngine(backend.URL), "fixture", "windows-uia")
			if (err != nil) != forged || calls != 2 {
				t.Fatalf("calls=%d forged=%v err=%v", calls, forged, err)
			}
		})
	}
}

// Explicit local model endpoint only. No dispatch, host grants, task creation,
// personal document access or model downloads occur in this synthetic check.
func TestNativePreflightInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit local installed model")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "http" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		t.Fatal("requires literal loopback endpoint without credentials")
	}
	engine := inference.NewLlamaHTTPEngine(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	traced := &syntheticPreflightTrace{Engine: engine, engine: engine, t: t}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		err = probeComputerToolsDriver(ctx, traced, model, "windows-uia")
		cancel()
		if err != nil {
			t.Fatalf("synthetic native preflight repeat %d: %v", i+1, err)
		}
	}
}

type syntheticPreflightTrace struct {
	inference.Engine
	engine *inference.LlamaHTTPEngine
	t      *testing.T
}

func (e *syntheticPreflightTrace) ChatCompletionStreamRaw(ctx context.Context, req *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	return e.engine.ChatCompletionStreamRaw(ctx, req, func(data json.RawMessage) error {
		e.t.Logf("synthetic preflight only: %s", data)
		return callback(data)
	})
}
