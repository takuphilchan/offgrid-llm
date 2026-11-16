package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func policyWithDistantHeader(t *testing.T) []byte {
	t.Helper()
	row := func(a, b string) string {
		return `<w:tr><w:tc><w:p><w:r><w:t>` + a + `</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>` + b + `</w:t></w:r></w:p></w:tc></w:tr>`
	}
	header := strings.Replace(row("Category", "Annual cap (USD)"), `<w:tr>`, `<w:tr><w:trPr><w:tblHeader/></w:trPr>`, 1)
	data := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:tbl>` + header + strings.Repeat(row("Other category", "100"), 30) + row("Rural clinics", "17.25") + `</w:tbl></w:body></w:document>`
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func verifyLinkedHeaderHTTP(t *testing.T, s *Server, ctx context.Context, found SearchResponse) {
	t.Helper()
	linked := false
	for _, r := range found.Results {
		if !strings.HasPrefix(r.Chunk.ID, "linked-") || !strings.Contains(r.Chunk.Content, "Annual cap (USD)") {
			continue
		}
		linked = true
		c := r.Chunk
		if r.EvidenceReceipt == nil || !r.EvidenceReceipt.LinksComplete || r.EvidenceReceipt.Revision == "" {
			t.Fatal("missing structural receipt")
		}
		q := url.Values{"id": {r.DocumentID}, "start_byte": {strconv.Itoa(c.StartChar)}, "end_byte": {strconv.Itoa(c.EndChar)}, "source_sha256": {c.Source.SHA256}, "extractor_revision": {c.Source.Revision}, "content_hash": {c.ContentHash}}
		w := httptest.NewRecorder()
		s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?"+q.Encode(), nil).WithContext(ctx))
		var result struct {
			Content  string
			Verified bool `json:"citation_verified"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || !result.Verified || result.Content != c.Content {
			t.Fatal("linked header citation failed", w.Code, w.Body.String())
		}
	}
	if !linked {
		t.Fatal("real search omitted distant header")
	}
}

func TestCaptureKnowledgeRetainsIncompleteLinkedEvidence(t *testing.T) {
	result := &rag.RAGContext{RetrievalMode: "hybrid", Results: []rag.SearchResult{{DocumentID: "doc", DocName: "Policy", Chunk: &rag.Chunk{ID: "value", Content: "17.25"}, EvidenceReceipt: &document.EvidenceReceipt{Revision: "extraction-1", GroupID: "value", RequiredChunks: []string{"value", "header"}, LinksComplete: true}}}}
	result.FormatContext()
	var saved []sessions.KnowledgePassage
	captureKnowledge(withKnowledgeScope(context.Background(), []string{"doc"}, &saved), result)
	if len(saved) != 1 || saved[0].EvidenceReceipt == nil || saved[0].EvidenceReceipt.LinksComplete || saved[0].EvidenceReceipt.RequiredChunks[1] != "header" {
		t.Fatal("captured evidence lost missing header", saved)
	}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var loaded []sessions.KnowledgePassage
	if json.Unmarshal(data, &loaded) != nil || loaded[0].EvidenceReceipt.LinksComplete {
		t.Fatal("saved receipt changed")
	}
}
