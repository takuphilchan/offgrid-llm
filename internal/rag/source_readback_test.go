package rag

import (
	"strings"
	"testing"
)

func TestSourcePassageExactReadbackAndStaleRejection(t *testing.T) {
	prefix := strings.Repeat("a", 200010) + "\n"
	text := "café 日本語\nline two"
	digest := strings.Repeat("a", 64)
	doc := &Document{SourceRetained: true, RawContent: prefix + text, SourceRanges: []SourceRange{
		{Kind: "page", Index: 1, Start: 0, End: len(prefix) - 1, SHA256: digest, Revision: "native"},
		{Kind: "page", Index: 2, Start: len(prefix), End: len(prefix) + len(text), SHA256: digest, Revision: "ocr"},
	}}
	start, end := len(prefix), len(doc.RawContent)
	hash := GenerateContentHash(text)
	got, err := doc.ReadSourcePassage(start, end, digest, "ocr", hash)
	if err != nil || got != text {
		t.Fatalf("late Unicode passage: %q %v", got, err)
	}
	for _, tc := range []struct {
		name                   string
		start, end             int
		digest, revision, hash string
	}{
		{"changed original", start, end, strings.Repeat("b", 64), "ocr", hash},
		{"changed extractor", start, end, digest, "new", hash},
		{"changed content", start, end, digest, "ocr", GenerateContentHash("other")},
		{"crosses page", start - 2, end, digest, "ocr", GenerateContentHash(doc.RawContent[start-2 : end])},
		{"splits Unicode", start + 4, end, digest, "ocr", GenerateContentHash(doc.RawContent[start+4 : end])},
		{"negative", -1, end, digest, "ocr", hash},
		{"past end", start, end + 1, digest, "ocr", hash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := doc.ReadSourcePassage(tc.start, tc.end, tc.digest, tc.revision, tc.hash); err == nil || got != "" {
				t.Fatal("invalid citation delivered", got, err)
			}
		})
	}
	doc.SourceRetained = false
	if _, err := doc.ReadSourcePassage(start, end, digest, "ocr", hash); err == nil {
		t.Fatal("removed source delivered")
	}
}

func TestCompleteExtractionRejectsIncompleteUnitsAndMixedOriginals(t *testing.T) {
	for _, state := range []string{"ocr_partial", "ocr_required", "empty", "unknown"} {
		t.Run(state, func(t *testing.T) {
			result := &ParseResult{State: ExtractionComplete, Content: "text", SourceUnits: []SourceUnit{{Kind: "page", Index: 1, State: state, Text: "text", SourceSHA256: strings.Repeat("a", 64), ExtractorRevision: "fixture"}}}
			if _, _, err := parsedSourceRanges(result); err == nil {
				t.Fatal("incomplete unit marked ready")
			}
		})
	}
	ranges := []SourceRange{{Kind: "page", Index: 1, Start: 0, End: 1, SHA256: strings.Repeat("a", 64), Revision: "native"}, {Kind: "page", Index: 2, Start: 1, End: 2, SHA256: strings.Repeat("b", 64), Revision: "ocr"}}
	if err := validateSourceRanges("ab", ranges); err == nil {
		t.Fatal("mixed originals accepted")
	}
	ranges[1].SHA256 = ranges[0].SHA256
	if err := validateSourceRanges("ab", ranges); err != nil {
		t.Fatal("native and OCR revisions must coexist", err)
	}
}
