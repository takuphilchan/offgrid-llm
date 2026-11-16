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

// Authored synthetic data, not a licensed external record or an answer benchmark.
func workbookWithDistantHeader(t *testing.T) []byte {
	t.Helper()
	var rows strings.Builder
	rows.WriteString(`<row r="1"><c r="A1" t="inlineStr"><is><t>Category</t></is></c><c r="B1" t="inlineStr"><is><t>Annual cap (USD)</t></is></c></row>`)
	for i := 2; i <= 300; i++ {
		fmt.Fprintf(&rows, `<row r="%d"><c r="A%d" t="inlineStr"><is><t>Other category</t></is></c><c r="B%d"><v>100</v></c></row>`, i, i, i)
	}
	rows.WriteString(`<row r="301"><c r="A301" t="inlineStr"><is><t>Zirconium clinics</t></is></c><c r="B301" s="1"><v>1234.567</v></c></row>`)
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, data := range map[string]string{
		"xl/workbook.xml":                    `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Allocations" r:id="s"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":         `<Relationships><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet.xml"/><Relationship Id="style" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":                      `<styleSheet><numFmts><numFmt numFmtId="164" formatCode="&quot;USD &quot;#,##0.00"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet.xml":            `<worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData>` + rows.String() + `</sheetData><tableParts><tablePart r:id="t"/></tableParts></worksheet>`,
		"xl/worksheets/_rels/sheet.xml.rels": `<Relationships><Relationship Id="t" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table.xml"/></Relationships>`,
		"xl/tables/table.xml":                `<table ref="A1:B301"/>`,
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

func TestXLSXEvidenceThroughPrivateHTTPGenerationAndSavedCitation(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "allocations.xlsx", string(workbookWithDistantHeader(t)))
	var upload struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &upload) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	a := upload.Attachment
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/structure?revision=%s&offset=600&limit=2", a.ID, a.Revision), nil))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Blocks) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	cell := page.Blocks[1].Cell
	if cell == nil || cell.Reference != "B301" || cell.Value != "1234.567" || cell.Display != "USD 1,234.57" || page.CanCorrect {
		t.Fatal("typed private reader lost native value", cell)
	}
	calls := 0
	h.streamer = func(_ context.Context, req *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		input := req.Messages[len(req.Messages)-1].StringContent()
		for _, want := range []string{"Zirconium clinics", "1234.567", "USD 1,234.57", "Annual cap (USD)", `"links_complete":true`} {
			if !strings.Contains(input, want) {
				t.Errorf("model context lost %s", want)
			}
		}
		return ChatStreamResult{Answer: "Transport fixture", FinishReason: "stop"}, nil
	}
	body, _ := json.Marshal(sessionGenerateRequest{Content: "What is the cap for Zirconium clinics?", Stream: true, Durable: true, RequestID: "xlsx-fidelity", Attachments: []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}})
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/generate", bytes.NewReader(body)))
	if w.Code != 200 || calls != 1 {
		t.Fatal(w.Code, w.Body.String(), calls)
	}
	saved, err := h.manager.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	selection := saved.Messages[len(saved.Messages)-1].AttachmentSelections
	if len(selection) != 1 || selection[0].LinkedEvidence == nil || !selection[0].LinkedEvidence.LinksComplete {
		t.Fatal("saved receipt lost dependencies")
	}
	foundValue, foundHeader := false, false
	for _, span := range selection[0].Ranges {
		w = httptest.NewRecorder()
		h.HandleSessions(w, httptest.NewRequest("GET", fmt.Sprintf("/v1/sessions/chat/attachments/%s/passage?revision=%s&start=%d&end=%d", a.ID, a.Revision, span[0], span[1]), nil))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		foundValue = foundValue || strings.Contains(w.Body.String(), "1234.567")
		foundHeader = foundHeader || strings.Contains(w.Body.String(), "Annual cap (USD)")
	}
	if !foundValue || !foundHeader {
		t.Fatal("saved source cannot reopen native value and distant header")
	}
}
