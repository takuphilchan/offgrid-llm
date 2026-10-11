//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestPromptTokensCountsFullRequestWithoutGeneration(t *testing.T) {
	parallel := false
	request := &api.ChatCompletionRequest{Model: "any-installed-model", Stream: true, ToolChoice: "auto", ParallelToolCalls: &parallel,
		Messages: []api.ChatMessage{{Role: "user", Content: "旅行 \"quoted\""}},
		Tools:    []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "fixture", Description: "Must count schemas too", Parameters: map[string]interface{}{"type": "object"}}}},
	}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions/input_tokens" || r.Method != http.MethodPost {
			t.Error("generation or unexpected endpoint", r.URL.Path)
		}
		var got api.ChatCompletionRequest
		if json.NewDecoder(r.Body).Decode(&got) != nil || len(got.Tools) != 1 || got.Tools[0].Function.Description != request.Tools[0].Function.Description || got.Messages[0].StringContent() != request.Messages[0].StringContent() || got.ToolChoice != "auto" || got.ParallelToolCalls == nil || *got.ParallelToolCalls || got.Model != "" || got.Stream {
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
