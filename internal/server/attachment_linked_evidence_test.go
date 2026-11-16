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

func longLinkedPolicy(t *testing.T) []byte {
	t.Helper()
	paragraph := `<w:p><w:r><w:t>Ordinary unrelated correspondence.</w:t></w:r></w:p>`
	body := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Procurement policy</w:t></w:r></w:p>` + strings.Repeat(paragraph, 400) + `<w:p><w:r><w:t>Zirconium applications close on 30 June.</w:t><w:footnoteReference w:id="1"/></w:r></w:p>` + strings.Repeat(paragraph, 400) + `</w:body></w:document>`
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	for name, content := range map[string]string{"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="Heading1"><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style></w:styles>`, "word/document.xml": body, "word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:footnote w:id="1"><w:p><w:r><w:t>Island applicants may apply until 31 July.</w:t></w:r></w:p></w:footnote></w:footnotes>`} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestAttachmentLinkedEvidenceThroughHTTPAndSavedCitation(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "policy.docx", string(longLinkedPolicy(t)))
	var upload struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &upload) != nil || upload.Attachment.ExtractionRevision == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	a := upload.Attachment
	structurePath := fmt.Sprintf("/v1/sessions/chat/attachments/%s/structure?revision=%s&limit=10", a.ID, a.Revision)
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", structurePath, nil))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || page.Revision != a.ExtractionRevision || page.CanCorrect || len(page.Blocks) != 10 || page.NextOffset == nil || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private structure inspection", w.Code, w.Body.String())
	}
	for _, b := range page.Blocks {
		if b.Editable {
			t.Fatal("private inspection enabled mutation")
		}
	}
	if page.Navigation == nil || page.NavigationOffset != 0 || page.Units[0].BlockCount < 800 {
		t.Fatal("private reader omitted navigation", page)
	}
	if len(page.Navigation) == 0 || page.Navigation[0].Title != "Procurement policy" {
		t.Fatal("DOCX heading missing from HTTP navigation", page.Navigation)
	}
	target := page.Navigation[0]
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", structurePath+"&unit="+target.UnitID+"&offset="+fmt.Sprint(target.BlockOffset), nil))
	var jumped document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &jumped) != nil || len(jumped.Blocks) == 0 || jumped.Blocks[0].ID != target.BlockID || jumped.Revision != page.Revision {
		t.Fatal("navigation opened different evidence", w.Code, w.Body.String())
	}
	for _, suffix := range []string{"&offset=-1", "&limit=201", "&unit=missing", "&relationship_offset=400001", "&navigation_offset=-1", "&navigation_offset=200001", "&navigation_offset=bad"} {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", strings.Replace(structurePath, "&limit=10", "", 1)+suffix, nil))
		if w.Code != 400 {
			t.Fatal("unbounded structure query", w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", strings.Replace(structurePath, a.Revision, "stale", 1), nil))
	if w.Code == 200 {
		t.Fatal("stale attachment structure accepted")
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		for _, want := range []string{"30 June", "31 July", `"links_complete":true`, `"selection":"excerpts"`} {
			if !strings.Contains(input, want) {
				t.Errorf("model input lost %s", want)
			}
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	request := sessionGenerateRequest{Content: "When do Zirconium applications close?", Stream: true, Durable: true, RequestID: "linked-evidence", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}}
	data, _ := json.Marshal(request)
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(data)))
	if w.Code != 200 || calls != 1 {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
	saved, err := h.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	last := saved.Messages[len(saved.Messages)-1]
	if len(last.AttachmentSelections) != 1 || last.AttachmentSelections[0].LinkedEvidence == nil || !last.AttachmentSelections[0].LinkedEvidence.LinksComplete {
		t.Fatal("saved receipt missing")
	}
	found := false
	for _, span := range last.AttachmentSelections[0].Ranges {
		path := fmt.Sprintf("/v1/sessions/chat/attachments/%s/passage?revision=%s&start=%d&end=%d", a.ID, a.Revision, span[0], span[1])
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "31 July") {
			found = true
		}
	}
	if !found {
		t.Fatal("exception cannot be opened from saved answer")
	}
}
