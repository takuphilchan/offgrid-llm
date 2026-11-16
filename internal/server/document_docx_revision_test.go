package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func trackedPolicy(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Application limit: </w:t></w:r><w:del w:id="1" w:author="Policy reviewer" w:date="2026-01-02T00:00:00Z"><w:r><w:delText>999.99 superseded</w:delText></w:r></w:del><w:ins w:id="2"><w:r><w:t>17.25 current</w:t></w:r></w:ins></w:p></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestDOCXRevisionsThroughHTTPAndChat(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "tracked.docx", string(trackedPolicy(t)))
	var uploaded struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if (w.Code != 200 && w.Code != 201) || json.Unmarshal(w.Body.Bytes(), &uploaded) != nil || uploaded.Attachment.Status != "ready" {
		t.Fatal(w.Code, w.Body.String())
	}
	a := uploaded.Attachment
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", "/v1/sessions/chat/attachments/"+a.ID+"/structure?revision="+a.Revision, nil))
	var view document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil || len(view.Blocks) != 1 || len(view.Blocks[0].Revisions) != 2 || view.Blocks[0].Revisions[0].Text != "999.99 superseded" || strings.Contains(view.Blocks[0].Text, "999.99") {
		t.Fatal("revision inspection lost fidelity", w.Code, w.Body.String())
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		if !strings.Contains(input, "17.25 current") || strings.Contains(input, "999.99") || !strings.Contains(input, "document_tracked_changes") {
			t.Error("current context lost revision boundary")
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "What is the application limit?", Stream: true, Durable: true, RequestID: "tracked-evidence", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
	if w.Code != 200 || calls != 1 {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
}
