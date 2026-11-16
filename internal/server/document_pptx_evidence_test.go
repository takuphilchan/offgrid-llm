package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func presentationWithNotesException(t *testing.T) []byte {
	t.Helper()
	ns := `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	shape := func(text string) string {
		return `<p:sp><p:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, data := range map[string]string{
		"ppt/presentation.xml":            `<p:presentation ` + ns + `><p:sldIdLst><p:sldId r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide.xml"/></Relationships>`,
		"ppt/slides/slide.xml":            `<p:sld ` + ns + `><p:cSld><p:spTree>` + shape("Zirconium applications close on 30 June.") + strings.Repeat(shape("Ordinary unrelated correspondence."), 300) + `</p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide.xml.rels": `<Relationships><Relationship Id="n" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notes.xml"/></Relationships>`,
		"ppt/notesSlides/notes.xml":       `<p:notes ` + ns + `><p:cSld><p:spTree>` + shape("Island applicants may apply until 31 July.") + `</p:spTree></p:cSld></p:notes>`,
	} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPPTXNotesThroughHTTPContextAndSavedCitation(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "policy.pptx", string(presentationWithNotesException(t)))
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	a := result.Attachment
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/structure?revision=%s&offset=301&limit=2", a.ID, a.Revision), nil))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Blocks) != 2 || page.Blocks[0].SourcePart != "ppt/notesSlides/notes.xml" || page.Blocks[0].ReviewState != "speaker_notes" {
		t.Fatal(w.Code, w.Body.String())
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		for _, want := range []string{"30 June", "31 July", "[Speaker note]", `"links_complete":true`} {
			if !strings.Contains(input, want) {
				t.Errorf("model input lost %s", want)
			}
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "When do Zirconium applications close?", Stream: true, Durable: true, RequestID: "pptx-notes", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
	if w.Code != 200 || calls != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := h.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	selections := saved.Messages[len(saved.Messages)-1].AttachmentSelections
	if len(selections) != 1 || selections[0].LinkedEvidence == nil || !selections[0].LinkedEvidence.LinksComplete {
		t.Fatal("missing saved link receipt")
	}
	found := false
	for _, span := range selections[0].Ranges {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/passage?revision=%s&start=%d&end=%d", a.ID, a.Revision, span[0], span[1]), nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		found = found || strings.Contains(w.Body.String(), "31 July")
	}
	if !found {
		t.Fatal("saved citation lost notes exception")
	}
}
