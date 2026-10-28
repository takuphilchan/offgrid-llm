//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestPromptTokensRetriesOnlyReadOnlyTransportEOFOnce(t *testing.T) {
	for _, tc := range []struct{ alwaysDrop, reset bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		var calls atomic.Int32
		endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions/input_tokens" {
				t.Error("unexpected generation request")
			}
			n := calls.Add(1)
			if tc.alwaysDrop || n == 1 {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				if tcp, ok := conn.(*net.TCPConn); ok && tc.reset {
					_ = tcp.SetLinger(0)
				}
				conn.Close()
				return
			}
			w.Write([]byte(`{"input_tokens":123}`))
		}))
		count, err := NewLlamaHTTPEngine(endpoint.URL).CountPromptTokens(context.Background(), &api.ChatCompletionRequest{Messages: []api.ChatMessage{{Role: "user", Content: "fixture"}}})
		endpoint.Close()
		if calls.Load() != 2 {
			t.Fatal("unbounded/missing retry", tc, calls.Load(), err)
		}
		if tc.alwaysDrop && err == nil || !tc.alwaysDrop && (err != nil || count != 123) {
			t.Fatal(count, err)
		}
	}
}

func TestPromptTokensCountsFullRequestWithoutGeneration(t *testing.T) {
	parallel := false
	request := &api.ChatCompletionRequest{SingleStepTools: true, Model: "any-installed-model", Stream: true, ToolChoice: "auto", ParallelToolCalls: &parallel,
		Messages: []api.ChatMessage{{Role: "user", Content: "旅行 \"quoted\""}},
		Tools:    []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "fixture", Description: "Must count schemas too", Parameters: map[string]interface{}{"type": "object"}}}},
	}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions/input_tokens" || r.Method != http.MethodPost {
			t.Error("generation or unexpected endpoint", r.URL.Path)
		}
		var wire map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&wire) != nil || wire["response_format"] == nil {
			t.Fatal("single-step grammar absent")
		}
		delete(wire, "response_format")
		data, _ := json.Marshal(wire)
		var got api.ChatCompletionRequest
		if json.Unmarshal(data, &got) != nil || len(got.Tools) != 0 || len(got.Messages) != 3 || !strings.Contains(got.Messages[1].StringContent(), request.Tools[0].Function.Description) || got.Messages[2].StringContent() != request.Messages[0].StringContent() || got.ToolChoice != nil || got.ParallelToolCalls == nil || *got.ParallelToolCalls || got.Model != "" || got.Stream {
			t.Errorf("count request diverged: %+v", got)
		}
		w.Write([]byte(`{"input_tokens":1234,"object":"response.input_tokens"}`))
	}))
	defer endpoint.Close()
	count, err := NewLlamaHTTPEngine(endpoint.URL).CountPromptTokens(context.Background(), request)
	if err != nil || count != 1234 || request.Model != "any-installed-model" || !request.Stream {
		t.Fatalf("count=%d error=%v request=%+v", count, err, request)
	}
}

func TestPromptTokensRejectsInvalidAndPrivateRuntimeErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		unsupported bool
	}{
		{"old", 404, "PRIVATE", true}, {"unimplemented", 501, "PRIVATE", true},
		{"busy", 503, "PRIVATE", false}, {"zero", 200, `{"input_tokens":0}`, false},
		{"missing", 200, `{}`, false}, {"negative", 200, `{"input_tokens":-1}`, false},
		{"huge", 200, `{"input_tokens":100000001}`, false}, {"malformed", 200, `null`, false},
		{"trailing", 200, `{"input_tokens":23} PRIVATE`, false}, {"oversized", 200, strings.Repeat("PRIVATE", 1000), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer endpoint.Close()
			count, err := NewLlamaHTTPEngine(endpoint.URL).CountPromptTokens(context.Background(), &api.ChatCompletionRequest{})
			if err == nil || count != 0 || strings.Contains(err.Error(), "PRIVATE") || errors.Is(err, ErrPromptCountUnavailable) != tc.unsupported {
				t.Fatalf("count=%d err=%v", count, err)
			}
		})
	}
}

func TestPromptTokensCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewLlamaHTTPEngine("http://127.0.0.1:1").CountPromptTokens(ctx, &api.ChatCompletionRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
