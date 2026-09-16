package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func newStreamingSessions(t *testing.T) *SessionHandlers {
	t.Helper()
	h := NewSessionHandlers(t.TempDir())
	if err := h.manager.Save(sessions.NewSession("chat", "model")); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSessionStreamFlushesBeforeGenerationAndSavesBeforeDone(t *testing.T) {
	h := newStreamingSessions(t)
	finish := make(chan struct{})
	h.streamer = func(ctx context.Context, req *api.ChatCompletionRequest, profile string, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
		if *req.MaxTokens != 256 || req.Messages[0].StringContent() != "hello" || profile != "interactive" {
			return ChatStreamResult{}, errors.New("wrong request")
		}
		if err := emit(ChatStreamEvent{Type: "delta", Delta: "Hello"}); err != nil {
			return ChatStreamResult{}, err
		}
		select {
		case <-finish:
		case <-ctx.Done():
			return ChatStreamResult{}, ctx.Err()
		}
		return ChatStreamResult{Answer: "Hello", FinishReason: "stop"}, nil
	}
	server := httptest.NewServer(http.HandlerFunc(h.HandleSessions))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/v1/sessions/chat/generate", strings.NewReader(`{"content":"hello","stream":true,"max_tokens":256,"profile":"interactive"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, `"queued"`) {
		t.Fatalf("no immediate phase: %s %v", line, err)
	}
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(line, `"delta"`) {
			break
		}
	}
	before, err := h.manager.Load("chat")
	if err != nil || len(before.Messages) != 0 {
		t.Fatalf("partial exchange persisted: %+v %v", before, err)
	}
	close(finish)
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rest), `"type":"done"`) {
		t.Fatalf("missing done: %s", rest)
	}
	saved, err := h.manager.Load("chat")
	if err != nil || len(saved.Messages) != 2 || saved.Messages[1].Content != "Hello" {
		t.Fatalf("not saved: %+v %v", saved, err)
	}
}

func TestFailedStreamNeverCommitsPartialExchange(t *testing.T) {
	h := newStreamingSessions(t)
	h.streamer = func(ctx context.Context, req *api.ChatCompletionRequest, profile string, emit func(ChatStreamEvent) error) (ChatStreamResult, error) {
		_ = emit(ChatStreamEvent{Type: "delta", Delta: "partial"})
		return ChatStreamResult{Answer: "partial"}, io.ErrUnexpectedEOF
	}
	w := httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(`{"content":"hello","stream":true}`)))
	if !strings.Contains(w.Body.String(), `"type":"error"`) || strings.Contains(w.Body.String(), `"type":"done"`) {
		t.Fatalf("false completion: %s", w.Body.String())
	}
	saved, err := h.manager.Load("chat")
	if err != nil || len(saved.Messages) != 0 {
		t.Fatalf("partial saved: %+v %v", saved, err)
	}
}

func TestSessionStreamCancellationWhileWaitingDoesNotRunInference(t *testing.T) {
	h := newStreamingSessions(t)
	h.streamer = func(context.Context, *api.ChatCompletionRequest, string, func(ChatStreamEvent) error) (ChatStreamResult, error) {
		t.Error("cancelled queued request ran inference")
		return ChatStreamResult{}, nil
	}
	unlock := h.lockSession("chat")
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(`{"content":"hello","stream":true}`)).WithContext(ctx)
	cancel()
	h.HandleSessions(httptest.NewRecorder(), req)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.locksMu.Lock()
		refs := h.sessionLocks["chat"].refs
		h.locksMu.Unlock()
		if refs == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("cancelled waiter leaked its lock reference")
}

func TestChatProfileDoesNotChangeAgentContext(t *testing.T) {
	s := &Server{config: &config.Config{MaxContextSize: 65536, ChatContextSize: 8192, AdaptiveContext: false}}
	if s.chatContextWindow("interactive") != 8192 || s.chatContextWindow("extended") != 65536 || s.effectiveContextWindow() != 65536 {
		t.Fatal("context profiles corrupted agent context")
	}
	s.config.MaxContextSize = 4096
	if s.chatContextWindow("interactive") != 4096 {
		t.Fatal("interactive exceeded service limit")
	}
}

func TestStreamOptionsValidatedBeforeInference(t *testing.T) {
	h := newStreamingSessions(t)
	for _, body := range []string{`{"content":"x","stream":true,"profile":"unknown"}`, `{"content":"x","stream":true,"max_tokens":-1}`, `{"content":"x","stream":true,"max_tokens":4097}`} {
		w := httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
}

func TestStreamEventEncodesPersistedSession(t *testing.T) {
	raw, err := json.Marshal(ChatStreamEvent{Type: "done", Session: sessions.NewSession("chat", "model")})
	if err != nil || !strings.Contains(string(raw), `"session"`) {
		t.Fatalf("%s %v", raw, err)
	}
}
