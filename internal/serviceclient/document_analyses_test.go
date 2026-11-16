package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestAnalysisClientPinsSourcesAndNeverReplaysUnconfirmedMutation(t *testing.T) {
	refs := []document.SourceRef{{DocumentID: "before", Revision: "old"}, {DocumentID: "after", Revision: "new"}}
	for _, mode := range []string{"valid", "wrong-source", "false-complete", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/document-analyses" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer key" {
					t.Error("wrong route or authority")
				}
				if mode == "conflict" {
					w.WriteHeader(409)
					return
				}
				result := rag.AnalysisRecord{ID: "analysis", State: "queued", Plan: &document.AnalysisPlan{ID: "plan", Version: document.AnalysisVersion, Workflow: "comparison", Sources: append([]document.SourceRef{}, refs...), Units: []document.AnalysisUnit{{Source: 0, UnitID: "u"}, {Source: 1, UnitID: "v"}}}, Pairs: []document.ComparisonPair{{BeforeUnit: "u", AfterUnit: "v"}}}
				if mode == "wrong-source" {
					result.Plan.Sources[0].Revision = "replacement"
				}
				if mode == "false-complete" {
					result.State = "complete"
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer s.Close()
			c, err := New(s.URL, "key", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.CreateDocumentComparison(context.Background(), "request", refs)
			if (err == nil) != (mode == "valid") || calls != 1 {
				t.Fatal(mode, calls, err)
			}
		})
	}
}

func TestAnalysisClientSourceChecksDigestAndIdentity(t *testing.T) {
	for _, mode := range []string{"valid", "digest", "wrong-side", "span"} {
		t.Run(mode, func(t *testing.T) {
			text := "政策 17.25"
			hash := sha256.Sum256([]byte(text))
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("id") != "private&id" || r.URL.Query().Get("side") != "before" {
					t.Error("scope lost")
				}
				result := rag.AnalysisSourceResult{AnalysisID: "private&id", DocumentID: "before", Text: text, Citation: document.AnalysisCitation{Revision: "old", Source: 0, Span: document.Span{End: len(text)}, TextSHA256: hex.EncodeToString(hash[:])}}
				if mode == "digest" {
					result.Text = "tampered"
				}
				if mode == "wrong-side" {
					result.Citation.Source = 1
				}
				if mode == "span" {
					result.Citation.Span.End++
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer s.Close()
			c, err := New(s.URL, "key", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.DocumentAnalysisSource(context.Background(), "private&id", 0, 0, "before")
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}
