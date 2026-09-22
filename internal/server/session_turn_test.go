package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDurableChatDisconnectReplayAndExplicitCancel(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	finish := make(chan struct{})
	started := make(chan struct{}, 2)
	var calls atomic.Int32
	h.streamer = func(ctx context.Context, _ *api.ChatCompletionRequest, _ string, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls.Add(1)
		if err := emit(ChatStreamEvent{Type: "delta", Delta: "partial"}); err != nil {
			return ChatStreamResult{}, err
		}
		started <- struct{}{}
		select {
		case <-finish:
			return ChatStreamResult{Answer: "complete", FinishReason: "stop"}, nil
		case <-ctx.Done():
			return ChatStreamResult{}, ctx.Err()
		}
	}
	server := httptest.NewServer(http.HandlerFunc(h.HandleSessions))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/v1/sessions/chat/generate", strings.NewReader(`{"content":"hello","stream":true,"durable":true,"request_id":"one"}`))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = bufio.NewReader(response.Body).ReadString('\n')
	cancel()
	response.Body.Close()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker not started")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		saved, _ := h.manager.Load("chat")
		if saved.Turn != nil && saved.Turn.Output == "partial" {
			if !saved.Turn.Active() {
				t.Fatal("disconnect cancelled work")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no persisted progress")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(finish)
	replay, err := http.Get(server.URL + "/v1/sessions/chat/turn/events?id=one")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(replay.Body)
	replay.Body.Close()
	if !strings.Contains(string(body), `"type":"done"`) {
		t.Fatal(string(body))
	}
	// Same request must replay, not call inference again.
	again, err := http.Post(server.URL+"/v1/sessions/chat/generate", "application/json", strings.NewReader(`{"content":"hello","stream":true,"durable":true,"request_id":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, again.Body)
	again.Body.Close()
	if calls.Load() != 1 {
		t.Fatal("duplicate generation", calls.Load())
	}
	saved, _ := h.manager.Load("chat")
	if len(saved.Messages) != 2 {
		t.Fatal("exchange not atomically committed", saved)
	}
	finish = make(chan struct{})
	next, err := http.Post(server.URL+"/v1/sessions/chat/generate", "application/json", strings.NewReader(`{"content":"next","stream":true,"durable":true,"request_id":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer next.Body.Close()
	<-started
	stopped, err := http.Post(server.URL+"/v1/sessions/chat/turn/cancel", "application/json", strings.NewReader(`{"id":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	stopped.Body.Close()
	body, _ = io.ReadAll(next.Body)
	if !strings.Contains(string(body), `"type":"error"`) {
		t.Fatal(string(body))
	}
	state, err := http.Get(server.URL + "/v1/sessions/chat/turn")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Body.Close()
	var result map[string]any
	json.NewDecoder(state.Body).Decode(&result)
	if result["turn"].(map[string]any)["status"] != "cancelled" {
		t.Fatal(result)
	}
	saved, _ = h.manager.Load("chat")
	if len(saved.Messages) != 2 {
		t.Fatal("cancelled text entered context")
	}
}
