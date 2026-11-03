package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestAttachmentPreviewAndTextOnlyAreExplicit(t *testing.T) {
	item := sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: "paper.pdf", SHA256: strings.Repeat("b", 64), Bytes: 100, Status: "failed"}
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("missing authentication")
		}
		if strings.HasSuffix(r.URL.Path, "/preview") {
			if r.Method != "GET" || r.URL.Query().Get("sha256") != item.SHA256 {
				t.Error("wrong read identity")
			}
			json.NewEncoder(w).Encode(map[string]any{"name": item.Name, "sha256": item.SHA256, "state": "partial", "units": []map[string]any{{"kind": "page", "index": 1, "text": "café 日本語\x1b[2J", "state": "ocr_required", "source_sha256": item.SHA256}}})
			return
		}
		writes++
		if r.Method != "POST" || r.URL.Query().Get("text_only") != "true" {
			t.Error("implicit acceptance")
		}
		accepted := item
		accepted.Status = "ready"
		accepted.Coverage = "native_text_only"
		accepted.Revision = strings.Repeat("c", 64)
		json.NewEncoder(w).Encode(map[string]any{"attachment": accepted})
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "private", nil)
	draft := ConversationFileDraft{Files: []sessions.Attachment{item}}
	handled, text, err := draft.Command(context.Background(), client, "chat", "/preview-attachment 1")
	if !handled || err != nil || !strings.Contains(text, "café 日本語") || strings.Contains(text, "\x1b") || writes != 0 {
		t.Fatal(text, err, writes)
	}
	_, text, err = draft.Command(context.Background(), client, "chat", "/attachment-text-only 1")
	if err != nil || writes != 1 || draft.Files[0].Coverage != "native_text_only" || !strings.Contains(text, "unread") {
		t.Fatal(text, err, writes)
	}
}
