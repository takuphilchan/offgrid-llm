package serviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestSessionCorrectionCapabilityPinnedIdentityAndNoReplay(t *testing.T) {
	id := "file-" + strings.Repeat("a", 32)
	old := strings.Repeat("b", 64)
	next := strings.Repeat("c", 64)
	for _, mode := range []string{"ok", "legacy", "wrong", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private" {
					t.Error("missing auth")
				}
				if r.URL.Path == "/api/v2/system" {
					caps := []string{}
					if mode != "legacy" {
						caps = append(caps, "session-document-corrections-v1")
					}
					json.NewEncoder(w).Encode(Identity{Product: "offgrid", Capabilities: caps})
					return
				}
				writes++
				if r.Method != "POST" || r.URL.Path != "/v1/sessions/chat & policy/attachments/"+id+"/corrections" {
					t.Error("wrong mutation", r.URL.Path)
				}
				var body AttachmentCorrectionRequest
				if json.NewDecoder(r.Body).Decode(&body) != nil || body.ExpectedRevision != old || body.ExtractionRevision != "extraction-old" {
					t.Error("lost pinned identity")
				}
				if mode == "uncertain" {
					w.WriteHeader(503)
					return
				}
				item := sessions.Attachment{ID: id, Status: "ready", Revision: next, ParentRevision: old, ExtractionRevision: "extraction-next"}
				if mode == "wrong" {
					item.ParentRevision = next
				}
				json.NewEncoder(w).Encode(map[string]any{"attachment": item})
			}))
			defer s.Close()
			c, _ := New(s.URL, "private", nil)
			_, err := c.CorrectSessionAttachment(context.Background(), "chat & policy", id, AttachmentCorrectionRequest{ExpectedRevision: old, ExtractionRevision: "extraction-old", Edits: []document.Correction{{BlockID: "note", ExpectedSHA256: old, Text: "corrected"}}})
			if (err == nil) != (mode == "ok") {
				t.Fatal(mode, err)
			}
			want := 1
			if mode == "legacy" {
				want = 0
			}
			if writes != want {
				t.Fatal("unexpected replay or downgrade", writes)
			}
		})
	}
}
