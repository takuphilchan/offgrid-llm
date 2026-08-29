package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSessionGeneratePersistsExchange(t *testing.T) {
	handlers := NewSessionHandlers(filepath.Join(t.TempDir(), "sessions"))
	if err := handlers.manager.Save(sessions.NewSession("chat", "model-a")); err != nil {
		t.Fatal(err)
	}
	handlers.SetCompleter(func(_ context.Context, model string, messages []api.ChatMessage, knowledge bool) (string, error) {
		if model != "model-a" || knowledge || len(messages) != 1 || messages[0].StringContent() != "hello" {
			t.Fatalf("unexpected completion request: model=%q knowledge=%v messages=%#v", model, knowledge, messages)
		}
		return "hi there", nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/chat/generate", bytes.NewBufferString(`{"content":"hello"}`))
	w := httptest.NewRecorder()
	handlers.HandleSessions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var response struct {
		Session sessions.Session `json:"session"`
		Message sessions.Message `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Session.Messages) != 2 || response.Message.Content != "hi there" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestSessionGenerateDoesNotPersistFailedTurn(t *testing.T) {
	handlers := NewSessionHandlers(filepath.Join(t.TempDir(), "sessions"))
	if err := handlers.manager.Save(sessions.NewSession("chat", "model-a")); err != nil {
		t.Fatal(err)
	}
	handlers.SetCompleter(func(context.Context, string, []api.ChatMessage, bool) (string, error) {
		return "", newServiceError(http.StatusServiceUnavailable, "runtime unavailable", nil)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/chat/generate", bytes.NewBufferString(`{"content":"hello"}`))
	w := httptest.NewRecorder()
	handlers.HandleSessions(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	loaded, err := handlers.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 0 {
		t.Fatalf("failed turn was persisted: %#v", loaded.Messages)
	}
}

func TestWriteServiceErrorDoesNotExposeInternalCause(t *testing.T) {
	w := httptest.NewRecorder()
	writeServiceError(w, newServiceError(
		http.StatusServiceUnavailable,
		"inference runtime is unavailable",
		context.DeadlineExceeded,
	))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if bytes.Contains(w.Body.Bytes(), []byte(context.DeadlineExceeded.Error())) {
		t.Fatalf("response exposed internal cause: %s", w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("inference runtime is unavailable")) {
		t.Fatalf("response omitted user-facing message: %s", w.Body.String())
	}
}
