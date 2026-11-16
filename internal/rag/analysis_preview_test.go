package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAnalysisResultsExposeBoundedVerifiedPreviews(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	refs := analysisDocuments(t, s)
	r, err := s.CreateComparison("owner", "previews", refs)
	if err != nil {
		t.Fatal(err)
	}
	_, lease, err := s.beginAnalysis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	graphs, err := s.analysisGraphs("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := document.CompareUnit(context.Background(), graphs[0], graphs[1], r.Pairs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(lease, 0, result); err != nil {
		t.Fatal(err)
	}
	page, err := s.AnalysisResults("owner", r.ID, 0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Previews) != 1 || page.Previews[0].Before == nil || page.Previews[0].After != nil || !strings.Contains(page.Previews[0].Before.Text, "17.25") {
		t.Fatal(page)
	}
	preview := page.Previews[0].Before
	if preview.Truncated || preview.TextSHA256 != page.Result.Changes[0].Before.TextSHA256 || preview.Span != page.Result.Changes[0].Before.Span {
		t.Fatal("preview disconnected from citation", preview)
	}
	for _, text := range []string{strings.Repeat("政策", 200), strings.Repeat("a", 511) + "🙂", strings.Repeat("a", 512), ""} {
		p := analysisPreview(text, 7)
		digest := sha256.Sum256([]byte(p.Text))
		if len(p.Text) > 512 || !utf8.ValidString(p.Text) || p.Span.Start != 7 || p.Span.End != 7+len(p.Text) || p.Truncated != (len(p.Text) < len(text)) || p.TextSHA256 != hex.EncodeToString(digest[:]) || !strings.HasPrefix(text, p.Text) {
			t.Fatal(p)
		}
	}
}
