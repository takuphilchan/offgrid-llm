package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"unicode/utf8"
)

// A preview is a bounded projection of already validated source bytes. It is
// not stored in checkpoints and never substitutes for the full citation.
type AnalysisPreview struct {
	Text       string        `json:"text"`
	Span       document.Span `json:"span"`
	TextSHA256 string        `json:"text_sha256"`
	Truncated  bool          `json:"truncated"`
}
type AnalysisChangePreview struct {
	Before *AnalysisPreview `json:"before,omitempty"`
	After  *AnalysisPreview `json:"after,omitempty"`
}

func analysisPreview(text string, start int) *AnalysisPreview {
	end := min(len(text), 512)
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	hash := sha256.Sum256([]byte(text[:end]))
	return &AnalysisPreview{Text: text[:end], Span: document.Span{Start: start, End: start + end}, TextSHA256: hex.EncodeToString(hash[:]), Truncated: end < len(text)}
}
