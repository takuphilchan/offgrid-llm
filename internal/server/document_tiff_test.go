package server

import (
	"bytes"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTIFFPrivateHTTPRetainsPartialPageIdentity(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	data := tifffixture.TwoPages()
	w := uploadChatFile(t, h, "record.tiff", string(data))
	var upload struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 422 || json.Unmarshal(w.Body.Bytes(), &upload) != nil || upload.Attachment.ID == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	a := upload.Attachment
	path := "/v1/sessions/chat/attachments/" + a.ID
	for _, suffix := range []string{"/preview", "/preview?page=1", "/preview?page=2", "/original"} {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", path+suffix, nil))
		if w.Code != 200 {
			t.Fatal(suffix, w.Code, w.Body.String())
		}
		if suffix == "/original" && !bytes.Equal(w.Body.Bytes(), data) {
			t.Fatal("changed original")
		}
	}
	scope := h.scoped(httptest.NewRequest("GET", "/", nil))
	text := "Amount 42\n\nException 17"
	coverage := &document.Coverage{Format: "image_document", Status: "partial", TotalUnits: 2, ReadableUnits: 1, ReviewUnits: []int{2}}
	graph := &document.Extraction{SourceKind: "original_file", OriginalSHA256: a.SHA256, Format: "image_document", Method: "ocr", Text: text, Coverage: coverage,
		Units:  []document.Unit{{ID: "page-1", Kind: "page", Index: 1, Span: document.Span{End: 9}, State: "ready", ParserRevision: "test-tiff"}, {ID: "page-2", Kind: "page", Index: 2, Span: document.Span{Start: 11, End: 23}, State: "ocr_partial", ParserRevision: "test-tiff"}},
		Blocks: []document.Block{{ID: "line-1", UnitID: "page-1", Kind: "paragraph", Method: "ocr", Span: document.Span{End: 9}}, {ID: "line-2", UnitID: "page-2", Kind: "paragraph", Method: "ocr", Span: document.Span{Start: 11, End: 23}}}}
	if err := graph.Seal(); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.PrepareAttachmentStructure("chat", a.ID, text, "test-tiff", "document_ocr_required", "", coverage, graph); err != nil {
		t.Fatal(err)
	}
	if _, _, err := scope.ReadAttachment("chat", a.ID); err == nil {
		t.Fatal("failed OCR admitted without acknowledgement")
	}
	check := func() {
		t.Helper()
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", path+"/preview", nil))
		var preview struct {
			Units []struct {
				Index    int    `json:"index"`
				Text     string `json:"text"`
				State    string `json:"state"`
				Geometry struct {
					Width       int `json:"width"`
					Height      int `json:"height"`
					Orientation int `json:"orientation"`
				} `json:"image_geometry"`
			} `json:"units"`
			State string `json:"state"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || len(preview.Units) != 2 || preview.Units[0].Text != "Amount 42" || preview.Units[1].Text != "Exception 17" || preview.Units[1].State != "ocr_partial" || preview.State != "partial" {
			t.Fatal(w.Code, w.Body.String())
		}
		if preview.Units[0].Geometry.Width != 120 || preview.Units[0].Geometry.Height != 80 || preview.Units[0].Geometry.Orientation != 1 || preview.Units[1].Geometry.Orientation != 1 {
			t.Fatal("missing encoded pixel frame", w.Body.String())
		}
	}
	check()
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", path+"/prepare?text_only=true", nil))
	var accepted struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &accepted) != nil || accepted.Attachment.Coverage != "partial_text_only" || accepted.Attachment.ExtractionRevision == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	check()
	sources, err := scope.AttachmentSources("chat", []sessions.AttachmentReference{{ID: a.ID, Revision: accepted.Attachment.Revision}})
	if err != nil || sources[0].Structure == nil || sources[0].Structure.Units[1].Index != 2 {
		t.Fatal(sources, err)
	}
	prompt := sessions.AttachmentPrompt("What is the exception?", sources)
	if !strings.Contains(prompt, "Exception 17") || !strings.Contains(prompt, `"status":"partial"`) {
		t.Fatal("lost evidence limits", prompt)
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", path+"/preview?sha256="+strings.Repeat("0", 64), nil))
	if w.Code != 409 {
		t.Fatal("wrong source accepted", w.Code)
	}
}
