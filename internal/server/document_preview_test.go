package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestDocumentPreviewTextIsScopedAndReadOnly(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	uploaded := uploadChatFile(t, h, "notes.txt", "Exact café 日本語 <script>alert(1)</script>")
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if err := json.Unmarshal(uploaded.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	path := "/v1/sessions/chat/attachments/" + result.Attachment.ID + "/preview"
	for _, tc := range []struct {
		path string
		code int
	}{{path, 200}, {path + "?sha256=" + strings.Repeat("0", 64), 409}, {path + "?page=1", 400}, {strings.Replace(path, "/chat/", "/missing/", 1), 404}} {
		w := httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body)
		}
		if tc.code == 200 {
			var preview struct {
				Units []rag.SourceUnit `json:"units"`
				SHA   string           `json:"sha256"`
			}
			if json.Unmarshal(w.Body.Bytes(), &preview) != nil || len(preview.Units) != 1 || !strings.Contains(preview.Units[0].Text, "café 日本語") || preview.SHA != result.Attachment.SHA256 {
				t.Fatal(w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("preview cacheable")
			}
		}
	}
	saved, err := h.manager.Load("chat")
	if err != nil || len(saved.Messages) != 0 {
		t.Fatal("preview submitted work", err)
	}
}

func TestDocumentPreviewRechecksAccessAfterExtraction(t *testing.T) {
	w := httptest.NewRecorder()
	serveDocumentPreview(w, httptest.NewRequest("GET", "/preview", nil), rag.NewDocumentParser(), "private.txt", []byte("private content"), func() bool { return false })
	if w.Code != 404 || strings.Contains(w.Body.String(), "private content") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
