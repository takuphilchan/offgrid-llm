package rag

import (
	"strings"
	"testing"
)

func TestStructuredChunkingKeepsParagraphAndExactSource(t *testing.T) {
	paragraph := strings.Repeat("条件 ", 80) + "Exception: 17.25 applies."
	r, err := NewDocumentParser().Parse([]byte("First clause.\n"+paragraph+"\nLast clause."), "policy.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "policy", RawContent: text, SourceRanges: ranges, Extraction: r.Structure, SourceRetained: true}
	chunks, err := NewChunker(DefaultChunkingOptions()).chunkDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range chunks {
		if c.Content != text[c.StartChar:c.EndChar] {
			t.Fatal("fabricated chunk offsets")
		}
		read, err := d.ReadSourcePassage(c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
		if err != nil || read != c.Content {
			t.Fatal("source readback mismatch", err)
		}
		if strings.Contains(c.Content, paragraph) {
			found = true
		}
	}
	if !found {
		t.Fatal("ordinary paragraph split despite fitting bounded block budget")
	}
}
