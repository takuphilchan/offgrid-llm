package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestPrivatePDFNativeFormHTTP(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER") == "" {
		t.Skip("requires bundled PDF worker beside test executable")
	}
	data, err := os.ReadFile("../rag/testdata/documents/native-form.pdf")
	if err != nil {
		t.Fatal(err)
	}
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "form.pdf", string(data), "")
	var uploaded struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if (w.Code != 200 && w.Code != 201) || json.Unmarshal(w.Body.Bytes(), &uploaded) != nil || uploaded.Attachment.Status != "ready" {
		t.Fatal(w.Code, w.Body.String())
	}
	a := uploaded.Attachment
	if a.Evidence == nil || a.Evidence.Status != "text_only" || len(a.Evidence.ReviewUnits) != 1 {
		t.Fatal("form coverage lost", a)
	}
	base := "/v1/sessions/chat/attachments/" + a.ID
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", base+"/structure?revision="+a.Revision, nil))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Blocks) != 4 || page.Blocks[0].FormField == nil || page.Blocks[0].FormField.Value != "0017.25" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", base+"/preview?revision="+a.Revision, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Amount: 0017.25") || !strings.Contains(w.Body.String(), "[x] Yes") {
		t.Fatal(w.Code, w.Body.String())
	}
	calls := 0
	h.streamer = func(_ context.Context, request *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		var input string
		for _, message := range request.Messages {
			input += message.StringContent()
		}
		if !strings.Contains(input, "0017.25") || !strings.Contains(input, "document_form_review") {
			t.Error("Chat lost form evidence or limitations")
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "What is the amount?", Stream: true, Durable: true, RequestID: "native-form-evidence", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
	if w.Code != 200 || calls != 1 {
		t.Fatal("form Chat admission", w.Code, w.Body.String(), calls)
	}
}
