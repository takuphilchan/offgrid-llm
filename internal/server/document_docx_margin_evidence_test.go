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

func policyWithMarginException(t *testing.T) []byte {
	t.Helper()
	const ns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, data := range map[string]string{
		"word/document.xml":            `<w:document ` + ns + `><w:body><w:p><w:r><w:t>Applications close on 30 June.</w:t></w:r></w:p><w:sectPr><w:headerReference r:id="h"/><w:footerReference r:id="f"/></w:sectPr></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships><Relationship Id="h" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header.xml"/><Relationship Id="f" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer.xml"/></Relationships>`,
		"word/header.xml":              `<w:hdr ` + ns + `><w:p><w:r><w:t>Effective date: 2026-11-13.</w:t></w:r></w:p></w:hdr>`,
		"word/footer.xml":              `<w:ftr ` + ns + `><w:p><w:r><w:t>Rural applicants may apply until 31 July.</w:t></w:r></w:p></w:ftr>`,
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

func TestDOCXMarginsThroughHTTPContextAndSavedCitation(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "margin-policy.docx", string(policyWithMarginException(t)))
	var result struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	a := result.Attachment
	if a.Evidence == nil || a.Evidence.Status != "text_only" || a.Evidence.TotalUnits != 3 || len(a.Evidence.ReviewUnits) != 2 {
		t.Fatal("margin coverage lost", a.Evidence)
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/structure?revision=%s", a.ID, a.Revision), nil))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	found := false
	for _, b := range page.Blocks {
		if b.Kind == "footer" {
			found = b.SourcePart == "word/footer.xml" && b.UnitID == "section-3" && strings.Contains(b.Text, "31 July")
		}
	}
	if !found {
		t.Fatal("footer provenance missing", w.Body.String())
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		for _, want := range []string{"30 June", "31 July", "2026-11-13", "document_margin_layout"} {
			if !strings.Contains(input, want) {
				t.Errorf("model context lost %q", want)
			}
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "When can rural applicants apply?", Stream: true, Durable: true, RequestID: "docx-margins", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
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
	if len(selections) != 1 {
		t.Fatal("missing saved source selection")
	}
	found = false
	for _, span := range selections[0].Ranges {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/passage?revision=%s&start=%d&end=%d", a.ID, a.Revision, span[0], span[1]), nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		found = found || strings.Contains(w.Body.String(), "31 July")
	}
	if !found {
		t.Fatal("saved citation lost footer exception")
	}
}
