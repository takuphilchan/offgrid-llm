package serviceclient

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"unicode/utf8"
)

func validAnalysisPreviews(page rag.AnalysisResultPage) bool {
	if page.Previews == nil {
		return true
	} // Additive older-service compatibility.
	if page.Result == nil || len(page.Previews) != len(page.Result.Changes) {
		return false
	}
	for i, item := range page.Previews {
		change := page.Result.Changes[i]
		citations := []*document.AnalysisCitation{change.Before, change.After}
		for side, p := range []*rag.AnalysisPreview{item.Before, item.After} {
			c := citations[side]
			if (p == nil) != (c == nil) {
				return false
			}
			if p == nil {
				continue
			}
			hash := sha256.Sum256([]byte(p.Text))
			if !utf8.ValidString(p.Text) || len(p.Text) > 512 || p.Span.Start != c.Span.Start || p.Span.End < p.Span.Start || p.Span.End > c.Span.End || p.Span.End-p.Span.Start != len(p.Text) || p.Truncated != (p.Span.End < c.Span.End) || p.TextSHA256 != hex.EncodeToString(hash[:]) || (!p.Truncated && p.TextSHA256 != c.TextSHA256) {
				return false
			}
		}
	}
	return true
}
