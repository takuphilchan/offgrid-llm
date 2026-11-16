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
)

func verifyRealCorrectionHTTP(t *testing.T, s *Server, ctx context.Context, doc *rag.Document, original []byte, citation string) {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleDocumentStructure(w, httptest.NewRequest("GET", "/v1/documents/structure?id="+doc.ID, nil).WithContext(ctx))
	var page document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	var note *document.BlockView
	for i, b := range page.Blocks {
		if b.Kind == "footnote" {
			note = &page.Blocks[i]
			break
		}
	}
	if note == nil || !note.Editable {
		t.Fatal("linked note not available for review")
	}
	request := document.CorrectionRequest{DocumentID: doc.ID, ExpectedRevision: page.Revision, Edits: []document.Correction{{BlockID: note.ID, ExpectedSHA256: note.TextSHA256, Text: strings.ReplaceAll(note.Text, "31 July", "31 August")}}}
	body, _ := json.Marshal(request)
	w = httptest.NewRecorder()
	s.handleDocumentCorrections(w, httptest.NewRequest("POST", "/v1/documents/corrections", bytes.NewReader(body)).WithContext(ctx))
	var published IngestResponse
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &published) != nil || published.Document.ExtractionRevision == page.Revision {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?"+citation, nil).WithContext(ctx))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "31 July") || strings.Contains(w.Body.String(), "31 August") {
		t.Fatal("saved citation substituted", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?id="+doc.ID+"&format=original", nil).WithContext(ctx))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), original) {
		t.Fatal("correction modified original")
	}
	w = httptest.NewRecorder()
	s.handleDocumentStructure(w, httptest.NewRequest("GET", "/v1/documents/structure?id="+doc.ID, nil).WithContext(ctx))
	var updated document.StructurePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &updated) != nil || updated.ParentRevision != page.Revision || updated.Author != "local" || !strings.Contains(w.Body.String(), "31 August") {
		t.Fatal("new reviewed revision unavailable", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleDocumentCorrections(w, httptest.NewRequest("POST", "/v1/documents/corrections", bytes.NewReader(body)).WithContext(ctx))
	if w.Code != 409 {
		t.Fatal("stale correction replayed", w.Code, w.Body.String())
	}
}
