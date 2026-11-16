package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalysisClientPreviewIntegrity(t *testing.T) {
	for _, mode := range []string{"valid", "legacy", "text", "side", "span", "truncated", "full-digest"} {
		t.Run(mode, func(t *testing.T) {
			text := "政策 17.25"
			hash := sha256.Sum256([]byte(text))
			digest := hex.EncodeToString(hash[:])
			span := document.Span{Start: 4, End: 4 + len(text)}
			c := &document.AnalysisCitation{Source: 0, Revision: "old", UnitID: "page-1", Span: span, TextSHA256: digest}
			p := &rag.AnalysisPreview{Text: text, Span: span, TextSHA256: digest}
			result := rag.AnalysisResultPage{AnalysisID: "job", Step: 0, Offset: 0, TotalChanges: 1, Result: &document.UnitComparison{Changes: []document.ComparisonChange{{Kind: "text_removed", Before: c}}}, Previews: []rag.AnalysisChangePreview{{Before: p}}}
			switch mode {
			case "legacy":
				result.Previews = nil
			case "text":
				p.Text = "altered"
			case "side":
				result.Previews[0] = rag.AnalysisChangePreview{After: p}
			case "span":
				p.Span.Start++
			case "truncated":
				p.Truncated = true
			case "full-digest":
				c.TextSHA256 = "wrong"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(result) }))
			defer server.Close()
			client, e := New(server.URL, "key", server.Client())
			if e != nil {
				t.Fatal(e)
			}
			_, e = client.DocumentAnalysisResults(context.Background(), "job", 0, 0, 20)
			if (e == nil) != (mode == "valid" || mode == "legacy") {
				t.Fatal(mode, e)
			}
		})
	}
}
