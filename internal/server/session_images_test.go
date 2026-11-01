package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func testSessionImage(t *testing.T) api.SessionImage {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return api.SessionImage{Name: "café.png", DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())}
}
func postImageTurn(t *testing.T, h *SessionHandlers, req sessionGenerateRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.HandleSessionGenerate(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)), "chat")
	return w
}
func TestSessionImagesSavedReplayedAndIncludedInHistory(t *testing.T) {
	directory := t.TempDir()
	h := NewSessionHandlers(directory)
	if err := h.manager.Save(sessions.NewSession("chat", "model")); err != nil {
		t.Fatal(err)
	}
	defer h.closeTurns()
	h.imageReady = func(string) error { return nil }
	item := testSessionImage(t)
	var calls atomic.Int32
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		call := calls.Add(1)
		has, err := api.ValidateChatMessages(req.Messages)
		if err != nil || !has {
			t.Errorf("images lost: %v %v", has, err)
		}
		if call == 2 && len(req.Messages) != 3 {
			t.Errorf("history lost: %d", len(req.Messages))
		}
		return ChatStreamResult{Answer: "A fixture image", FinishReason: "stop"}, nil
	}
	req := sessionGenerateRequest{Images: []api.SessionImage{item}, Stream: true, Durable: true, RequestID: "image-one"}
	for i := 0; i < 2; i++ {
		w := postImageTurn(t, h, req)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"type":"done"`) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate inference", calls.Load())
	}
	saved, err := h.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Messages) != 2 || !reflect.DeepEqual(saved.Messages[0].Images, req.Images) || len(saved.Turn.Images) != 0 {
		t.Fatal("bytes not preserved exactly once")
	}
	req.Images[0].Name = "different.png"
	if w := postImageTurn(t, h, req); w.Code != 409 {
		t.Fatal("changed request reused", w.Code)
	}
	w := postImageTurn(t, h, sessionGenerateRequest{Content: "Describe again", Stream: true, Durable: true, RequestID: "image-two"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Independent load after constructing a fresh manager; no inference/replay.
	loaded, err := sessions.NewSessionManager(directory).Load("chat")
	if err != nil || len(loaded.Messages) != 4 || !reflect.DeepEqual(loaded.Messages[0].Images, []api.SessionImage{item}) {
		t.Fatal("restart lost images", err)
	}
	if calls.Load() != 2 {
		t.Fatal("restart replayed inference")
	}
}

func TestSessionImagesRejectUnsupportedPathsBeforeInference(t *testing.T) {
	for _, kind := range []string{"agent", "legacy", "model", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			h := newStreamingSessions(t)
			defer h.closeTurns()
			h.imageReady = func(string) error { return nil }
			h.streamer = func(context.Context, *api.ChatCompletionRequest, string, func(ChatStreamEvent) error) (ChatStreamResult, error) {
				t.Error("unexpected inference")
				return ChatStreamResult{}, nil
			}
			req := sessionGenerateRequest{Images: []api.SessionImage{testSessionImage(t)}, Stream: true, Durable: true, RequestID: "reject-image"}
			switch kind {
			case "agent":
				req.Agent = &sessions.AgentSelection{ID: "agent", Revision: 1}
			case "legacy":
				req.Durable = false
			case "model":
				h.imageReady = func(string) error { return newServiceError(422, "Unavailable vision", nil) }
			case "malformed":
				req.Images[0].DataURL = "file:///secret"
			}
			w := postImageTurn(t, h, req)
			if w.Code < 400 {
				t.Fatal(w.Code, w.Body.String())
			}
			saved, _ := h.manager.Load("chat")
			if saved.Turn != nil || len(saved.Messages) != 0 {
				t.Fatal("rejected input persisted")
			}
		})
	}
}
