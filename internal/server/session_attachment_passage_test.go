package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestSessionAttachmentPassageHTTP(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "note.txt", "café 日本語")
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/v1/sessions/chat/attachments/%s/passage", result.Attachment.ID)
	for _, tc := range []struct {
		query  string
		status int
	}{
		{fmt.Sprintf("?revision=%s&start=0&end=5", result.Attachment.Revision), 200},
		{"?start=0&end=5", 400},
		{"?revision=stale&start=0&end=5", 422},
		{fmt.Sprintf("?revision=%s&start=0&end=4", result.Attachment.Revision), 422},
	} {
		out := httptest.NewRecorder()
		h.HandleSessions(out, httptest.NewRequest("GET", base+tc.query, nil))
		if out.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.query, out.Code, out.Body)
		}
		if out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private passage could be cached")
		}
		if out.Code == 200 {
			var p sessions.AttachmentPassage
			_ = json.Unmarshal(out.Body.Bytes(), &p)
			if p.Text != "café" {
				t.Fatalf("wrong bytes: %+v", p)
			}
		}
	}
}
