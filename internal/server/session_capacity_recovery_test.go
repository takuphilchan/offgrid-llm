package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSessionCapacityFailureRetainsCodeWithoutReplay(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy-stream", true: "durable-stream"}[durable], func(t *testing.T) {
			h := newStreamingSessions(t)
			defer h.closeTurns()
			var calls atomic.Int32
			h.streamer = func(context.Context, *api.ChatCompletionRequest, string, func(ChatStreamEvent) error) (ChatStreamResult, error) {
				calls.Add(1)
				return ChatStreamResult{}, newServiceError(500, "load failed", &contextCapacityError{512, 0})
			}
			server := httptest.NewServer(http.HandlerFunc(h.HandleSessions))
			defer server.Close()
			request, _ := json.Marshal(map[string]any{"content": "retained draft", "stream": true, "durable": durable, "request_id": "capacity-test"})
			response, err := http.Post(server.URL+"/v1/sessions/chat/generate", "application/json", strings.NewReader(string(request)))
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || !strings.Contains(string(body), `"error_code":"insufficient_runtime_capacity"`) || strings.Contains(string(body), `"type":"done"`) {
				t.Fatalf("missing typed failure: %s (%v)", body, err)
			}
			saved, err := h.manager.Load("chat")
			if err != nil || len(saved.Messages) != 0 {
				t.Fatalf("failure changed completed context: %+v (%v)", saved, err)
			}
			if durable {
				if saved.Turn == nil || saved.Turn.Status != "failed" || saved.Turn.ErrorCode != "insufficient_runtime_capacity" || saved.Turn.Prompt != "retained draft" {
					t.Fatalf("failure not preserved: %+v", saved.Turn)
				}
				for _, path := range []string{"/turn", "/turn/events?id=capacity-test"} {
					replay, err := http.Get(server.URL + "/v1/sessions/chat" + path)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(replay.Body)
					replay.Body.Close()
					if err != nil || !strings.Contains(string(body), `"error_code":"insufficient_runtime_capacity"`) {
						t.Fatalf("recovery lost code: %s (%v)", body, err)
					}
				}
			}
			if calls.Load() != 1 {
				t.Fatal("failure inspection repeated inference", calls.Load())
			}
		})
	}
}
