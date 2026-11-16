package rag

import (
	"strings"
	"testing"
)

func TestEvidenceEvaluatorDoesNotCountRightDocumentWrongPassage(t *testing.T) {
	text := "Applications close 30 June. Exception: rural applicants may apply until 31 July."
	end := len(text)
	exception := strings.Index(text, "Exception")
	sha := strings.Repeat("a", 64)
	labels := []ExpectedEvidence{
		{DocumentID: "policy", SourceSHA256: sha, ExtractorRevision: "notes-v3", StartByte: 0, EndByte: exception, ContentHash: GenerateContentHash(text[:exception])},
		{DocumentID: "policy", SourceSHA256: sha, ExtractorRevision: "notes-v3", StartByte: exception, EndByte: end, ContentHash: GenerateContentHash(text[exception:]), Role: "exception"},
	}
	result := SearchResult{DocumentID: "policy", Chunk: &Chunk{Content: text[:exception], StartChar: 0, EndChar: exception, Source: &SourceRange{SHA256: sha, Revision: "notes-v3"}}}
	if err := validateExpectedEvidence(labels); err != nil {
		t.Fatal(err)
	}
	ranks, hits := scoreExpectedEvidence(labels, []SearchResult{result})
	if hits != 1 || ranks[1] != 0 {
		t.Fatal("omitted exception counted", ranks, hits)
	}
	result.Chunk.Content = text
	result.Chunk.EndChar = end
	if _, hits = scoreExpectedEvidence(labels, []SearchResult{result}); hits != 2 {
		t.Fatal("exact evidence not credited")
	}
	result.Chunk.Source.Revision = "other"
	if _, hits = scoreExpectedEvidence(labels, []SearchResult{result}); hits != 0 {
		t.Fatal("stale extraction counted")
	}
	result.Chunk.Source.Revision = "notes-v3"
	result.Chunk.Content = strings.ReplaceAll(text, "31 July", "30 July")
	if _, hits = scoreExpectedEvidence(labels, []SearchResult{result}); hits != 1 {
		t.Fatal("altered exception counted")
	}
}
