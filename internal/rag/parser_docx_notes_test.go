package rag

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDOCXFootnoteExceptionStaysWithClause(t *testing.T) {
	parts := map[string]string{
		"word/document.xml":  docxTestStart + `<w:p><w:r><w:t>Applications close on 30 June.</w:t><w:footnoteReference w:id="1"/></w:r></w:p>` + docxTestEnd,
		"word/footnotes.xml": `<w:footnotes xmlns:w="` + wordNamespace + `"><w:footnote w:id="1"><w:p><w:r><w:t>Exception: rural applicants may apply until 31 July.</w:t></w:r></w:p></w:footnote></w:footnotes>`,
	}
	p := NewDocumentParser()
	source := parserZip(t, parts)
	r, err := p.Parse(source, "policy.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "30 June. [footnote 1: Exception: rural applicants may apply until 31 July.") {
		t.Fatal(r.Content)
	}
	if r.SourceUnits[0].Text != r.Content || r.SourceUnits[0].SourceSHA256 == "" {
		t.Fatal("lost source identity")
	}
	// Synthetic orthogonal vectors make this a lexical transport regression,
	// not an embedding quality or model-answer evaluation.
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	e.enabled = true
	content, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	doc := &Document{ID: "policy", Name: "policy.docx", RawContent: content, SourceRetained: true, SourceRanges: ranges, Metadata: r.Metadata, IndexStatus: "ready", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunks, err := NewChunker(DefaultChunkingOptions()).chunkDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	vectors := make([][]float32, len(chunks))
	for i := range vectors {
		vectors[i] = []float32{0, 1}
	}
	if err = e.store.(*SQLiteStore).AddDocumentWithChunks(doc, chunks, vectors); err != nil {
		t.Fatal(err)
	}
	got, err := e.search(context.Background(), "When can rural applicants apply?", DefaultSearchOptions(), func(context.Context, []string) ([][]float32, error) { return [][]float32{{1, 0}}, nil })
	if err != nil || len(got.Results) == 0 || !strings.Contains(got.Context, "31 July") || !strings.Contains(got.Context, "document_text_only") {
		t.Fatalf("lost exception/coverage through Knowledge: %+v %v", got, err)
	}
	stored, err := e.store.GetDocument(doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	c := got.Results[0].Chunk
	read, err := stored.ReadSourcePassage(c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
	if err != nil || !strings.Contains(read, "31 July") {
		t.Fatal("citation lost exception", read, err)
	}
	encoded, _ := json.Marshal(stored)
	var listed Document
	if json.Unmarshal(encoded, &listed) != nil || listed.Evidence == nil || listed.Evidence.Status != "text_only" || listed.Evidence.Format != "docx" {
		t.Fatal("listing lost coverage", string(encoded))
	}
	delete(parts, "word/footnotes.xml")
	if _, err = p.Parse(parserZip(t, parts), "missing.docx", ".docx"); err == nil {
		t.Fatal("missing referenced note silently ignored")
	}
}

func TestDOCXNoteNamespaceZeroIDAndDeletedReferences(t *testing.T) {
	for _, namespace := range []string{wordNamespace, "http://purl.oclc.org/ooxml/wordprocessingml/main"} {
		parts := map[string]string{
			"word/document.xml": `<q:document xmlns:q="` + namespace + `"><q:body><q:p><q:r><q:t>Current clause</q:t><q:endnoteReference q:id="0"/></q:r><q:del><q:r><q:footnoteReference q:id="99"/></q:r></q:del></q:p></q:body></q:document>`,
			"word/endnotes.xml": `<q:endnotes xmlns:q="` + namespace + `"><q:endnote q:id="0"><q:p><q:r><q:t>Visible exception 17.25</q:t></q:r></q:p></q:endnote></q:endnotes>`,
		}
		got, err := NewDocumentParser().Parse(parserZip(t, parts), "tracked.docx", ".docx")
		if err != nil || !strings.Contains(got.Content, "17.25") || got.Metadata["tracked_changes"] == "" {
			t.Fatalf("namespace/track extraction: %+v %v", got, err)
		}
	}
}
