package rag

import (
	"testing"
	"time"
)

func TestStructuredExtractionPublicationAndRollback(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	parsed, err := NewDocumentParser().Parse([]byte("政策 17.25\nException applies."), "policy.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	d := &Document{ID: "policy", RawContent: text, SourceRanges: ranges, Metadata: parsed.Metadata, CreatedAt: now, UpdatedAt: now}
	if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
		t.Fatal(err)
	}
	first := d.ExtractionRevision
	graph, err := s.extraction(d.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Text != text || len(graph.Blocks) != 2 {
		t.Fatalf("lost structure: %+v", graph)
	}
	copy := *d
	copy.Extraction = nil
	copy.RawContent = "政策 18.25\nException applies."
	// A bad chunk makes publication fail after the new graph was inserted.
	bad := []*Chunk{{ID: "bad", DocumentID: "nonexistent"}}
	if err = s.ReplaceDocumentWithChunks(&copy, bad, [][]float32{{1}}); err == nil {
		t.Fatal("publication should fail")
	}
	current, err := s.GetDocument(d.ID)
	if err != nil || current.ExtractionRevision != first || current.RawContent != text {
		t.Fatal("failed publication replaced source", err)
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_extractions WHERE document_id=?`, d.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("orphan revision", count, err)
	}
	if err = s.ReplaceDocumentWithChunks(&copy, nil, nil); err != nil {
		t.Fatal(err)
	}
	graph, err = s.extraction(d.ID, first)
	if err != nil || graph.Text != text {
		t.Fatal("old revision lost", err)
	}
	if copy.ExtractionRevision == first {
		t.Fatal("replacement shares old revision")
	}
	if err = s.AddDocument(d); err == nil {
		t.Fatal("legacy replacement can erase history")
	}
	if err = s.DeleteDocument(d.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"document_extractions", "document_blocks", "document_relationships"} {
		if err = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("derived records survived deletion", table, count, err)
		}
	}
}

func TestStructuredExtractionQuotaRollsBackPublication(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.extractionQuota = 1
	d := &Document{ID: "quota", RawContent: "Source evidence must remain atomic."}
	if err = s.AddDocumentWithChunks(d, nil, nil); err == nil {
		t.Fatal("unbounded history publication")
	}
	got, err := s.GetDocument(d.ID)
	if err != nil || got != nil {
		t.Fatal("quota failure left a document", err)
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_extractions`).Scan(&count); err != nil || count != 0 {
		t.Fatal("quota failure left history", err)
	}
}
