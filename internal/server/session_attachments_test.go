package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func uploadChatFile(t *testing.T, h *SessionHandlers, name, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	f, err := form.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(f, content)
	_ = form.Close()
	r := httptest.NewRequest("POST", "/v1/sessions/chat/attachments", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	h.HandleSessions(w, r)
	return w
}

func TestConversationFileUploadAndDurableReply(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "notes.txt", "Source café 日本語. The shipment number is 12345.")
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var uploaded struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Attachment.Status != "ready" {
		t.Fatalf("not ready: %+v", uploaded)
	}
	refs := []sessions.AttachmentReference{{ID: uploaded.Attachment.ID, Revision: uploaded.Attachment.Revision}}
	var calls atomic.Int32
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls.Add(1)
		last := req.Messages[len(req.Messages)-1]
		if last.Role != "user" || !strings.Contains(last.StringContent(), "12345") || !strings.Contains(last.StringContent(), "untrusted content") {
			t.Errorf("source not represented as data: %+v", last)
		}
		return ChatStreamResult{Answer: "The shipment number is 12345 (notes.txt).", FinishReason: "stop"}, nil
	}
	request := sessionGenerateRequest{Content: "What is the shipment number?", Attachments: refs, Stream: true, Durable: true, RequestID: "files-one"}
	send := func() *httptest.ResponseRecorder {
		b, _ := json.Marshal(request)
		r := httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(b))
		w := httptest.NewRecorder()
		h.HandleSessions(w, r)
		return w
	}
	w = send()
	if w.Code != 200 || !strings.Contains(w.Body.String(), "12345") {
		t.Fatalf("reply: %d %s", w.Code, w.Body)
	}
	w = send()
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatalf("duplicate replay: %d calls=%d", w.Code, calls.Load())
	}
	saved, err := h.manager.Load("chat")
	if err != nil || len(saved.Messages) != 2 || len(saved.Messages[0].Attachments) != 1 || saved.Messages[0].Content != request.Content {
		t.Fatalf("history lost source or changed prompt: %+v %v", saved, err)
	}
	r := httptest.NewRequest(http.MethodDelete, "/v1/sessions/chat/attachments/"+refs[0].ID, nil)
	w = httptest.NewRecorder()
	h.HandleSessions(w, r)
	if w.Code != 409 {
		t.Fatalf("history source deleted: %d", w.Code)
	}
	request.Attachments[0].Revision = strings.Repeat("b", 64)
	w = send()
	if w.Code != 409 || calls.Load() != 1 {
		t.Fatalf("changed retry dispatched: %d", w.Code)
	}
	request.RequestID = "files-two"
	w = send()
	if w.Code != 422 || calls.Load() != 1 {
		t.Fatalf("changed source dispatched: %d", w.Code)
	}
}

func TestConversationFileFailureRetentionAndRemoval(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "broken.docx", "not a zip")
	if w.Code != 422 {
		t.Fatalf("corrupt document accepted: %d", w.Code)
	}
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result.Attachment.ID == "" || result.Attachment.Status != "failed" {
		t.Fatalf("failure lost source identity: %s", w.Body)
	}
	r := httptest.NewRequest("GET", "/v1/sessions/chat/attachments/"+result.Attachment.ID+"/original", nil)
	w = httptest.NewRecorder()
	h.HandleSessions(w, r)
	if w.Code != 200 || w.Body.String() != "not a zip" || w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("original recovery: %d", w.Code)
	}
	r = httptest.NewRequest("DELETE", "/v1/sessions/chat/attachments/"+result.Attachment.ID, nil)
	w = httptest.NewRecorder()
	h.HandleSessions(w, r)
	if w.Code != 200 {
		t.Fatalf("draft source removal: %d %s", w.Code, w.Body)
	}
	r = httptest.NewRequest("GET", "/v1/sessions/chat/attachments", nil)
	w = httptest.NewRecorder()
	h.HandleSessions(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"attachments":[]`) {
		t.Fatalf("removed source listed: %s", w.Body)
	}
	w = uploadChatFile(t, h, "program.exe", "plain text masquerading as executable")
	if w.Code != 422 {
		t.Fatal("unknown format guessed as text")
	}
}

func TestConversationCancelledUploadDoesNotCreateSource(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/v1/sessions/chat/attachments", strings.NewReader("cancelled multipart")).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleSessions(w, r)
	if w.Code != http.StatusRequestTimeout || !strings.Contains(w.Body.String(), "document_cancelled") {
		t.Fatalf("cancellation: %d %s", w.Code, w.Body)
	}
	items, err := h.manager.WithAccess(sessions.Access{All: true}).ListAttachments("chat")
	if err != nil || len(items) != 0 {
		t.Fatalf("cancelled upload created a source: %+v %v", items, err)
	}
}
