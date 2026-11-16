package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestPrivateCorrectionHTTPKeepsCitationsPreviewAndPublication(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "policy.txt", "Applications close in June.\nRural exception: July.", "")
	var uploaded struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 201 && w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if json.Unmarshal(w.Body.Bytes(), &uploaded) != nil {
		t.Fatal(w.Body.String())
	}
	a := uploaded.Attachment
	base := "/v1/sessions/chat/attachments/" + a.ID
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		w := httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest(method, path, bytes.NewReader(data)))
		return w
	}
	w = call("GET", base+"/structure?revision="+a.Revision+"&corrections=true", nil)
	var structure document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &structure) != nil || !structure.CanCorrect {
		t.Fatal(w.Code, w.Body.String())
	}
	block := structure.Blocks[len(structure.Blocks)-1]
	body := map[string]any{"expected_revision": a.Revision, "extraction_revision": structure.Revision, "edits": []document.Correction{{BlockID: block.ID, ExpectedSHA256: block.TextSHA256, Text: "Rural exception: August."}}}
	w = call("POST", base+"/corrections", body)
	var corrected struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &corrected) != nil || corrected.Attachment.ParentRevision != a.Revision {
		t.Fatal(w.Code, w.Body.String())
	}
	next := corrected.Attachment
	// Both revisions can occur in one durable conversation without replacing
	// the first message's evidence. This verifies the real HTTP admission path;
	// generation is a transport fixture, not a model-quality claim.
	calls := 0
	h.streamer = func(_ context.Context, request *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		var input string
		for _, message := range request.Messages {
			input += message.StringContent()
		}
		if !strings.Contains(input, "July") || calls == 2 && !strings.Contains(input, "August") {
			t.Error("conversation lost revision evidence")
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	for i, rev := range []string{a.Revision, next.Revision} {
		w = call("POST", "/v1/sessions/chat/generate", sessionGenerateRequest{Content: "What is the rural exception?", Stream: true, Durable: true, RequestID: []string{"before-correction", "after-correction"}[i], Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: rev}}})
		if w.Code != 200 {
			t.Fatal("conversation admission", w.Code, w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatal("conversation not exercised", calls)
	}
	if w = call("POST", base+"/corrections", body); w.Code != 409 {
		t.Fatal("stale edit", w.Code, w.Body.String())
	}
	for _, row := range []struct {
		rev, word string
		editable  bool
	}{{a.Revision, "July", false}, {next.Revision, "August", true}} {
		w = call("GET", base+"/preview?revision="+row.rev, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), row.word) {
			t.Fatal("wrong preview", w.Code, w.Body.String())
		}
		w = call("GET", base+"/structure?revision="+row.rev+"&corrections=true", nil)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &structure) != nil || structure.CanCorrect != row.editable {
			t.Fatal("historical editability", w.Code, w.Body.String())
		}
		w = call("GET", base+"/structure?revision="+row.rev, nil)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &structure) != nil || structure.CanCorrect {
			t.Fatal("legacy reader became editable", w.Code, w.Body.String())
		}
	}
	w = call("GET", base+"/revisions", nil)
	var history sessions.AttachmentRevisionHistory
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || history.CurrentRevision != next.Revision || len(history.Revisions) != 2 {
		t.Fatal("history", w.Code, w.Body.String())
	}
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	s := &Server{ragEngine: engine, sessionHandlers: h}
	publication := func(revision string) (*rag.ImportRecord, error) {
		body, _ := json.Marshal(map[string]any{"attachment": map[string]string{"session": "chat", "id": a.ID, "sha256": a.SHA256, "revision": revision}})
		r := httptest.NewRequest("POST", "/v1/collections/imports?collection_id=workspace&collection_revision=1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return s.acceptDocumentImport(r)
	}
	if _, err := publication(""); err == nil {
		t.Fatal("unversioned corrected publication silently reparsed")
	}
	accepted, err := publication(next.Revision)
	if err != nil {
		t.Fatal(err)
	}
	again, err := publication(next.Revision)
	if err != nil || again.ID != accepted.ID || accepted.ExtractionRevision != next.ExtractionRevision || again.ExtractionRevision != next.ExtractionRevision {
		t.Fatal("duplicate revision publication", err)
	}
	prior, err := publication(a.Revision)
	if err != nil || prior.ID == accepted.ID || prior.ExtractionRevision != a.ExtractionRevision {
		t.Fatal("distinct evidence revisions merged", err)
	}
}
