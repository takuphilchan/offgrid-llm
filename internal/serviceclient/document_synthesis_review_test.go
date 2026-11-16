package serviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestSynthesisReviewClientPinsReceiptAndNeverReplays(t *testing.T) {
	for _, mode := range []string{"valid", "digest", "revision", "decisions", "author", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			job, page := synthesisClientFixture()
			receipt := page.Receipts[0]
			posts := 0
			req := rag.SynthesisReviewRequest{ID: job.ID, Batch: 0, BatchID: receipt.BatchID, ResponseSHA256: receipt.ResponseSHA256, Decisions: []rag.SynthesisDecision{{Decision: "keep"}}, SectionChecked: true}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/results") {
					_ = json.NewEncoder(w).Encode(page)
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/review") {
					_ = json.NewEncoder(w).Encode(job)
					return
				}
				if r.Method == "POST" {
					posts++
				}
				v := rag.SynthesisReview{AnalysisID: job.ID, Batch: 0, BatchID: receipt.BatchID, ResponseSHA256: receipt.ResponseSHA256, Revision: 1, SectionChecked: true, Decisions: req.Decisions, Author: "owner", UpdatedAt: "2026-11-13T01:00:00Z"}
				switch mode {
				case "digest":
					v.ResponseSHA256 = strings.Repeat("f", 64)
				case "revision":
					v.Revision = 7
				case "decisions":
					v.Decisions = []rag.SynthesisDecision{{Decision: "exclude"}}
				case "author":
					v.Author = ""
				case "unknown":
					w.WriteHeader(503)
					return
				}
				_ = json.NewEncoder(w).Encode(v)
			}))
			defer s.Close()
			c, _ := New(s.URL, "key", s.Client())
			_, err := c.SaveDocumentSynthesisReview(context.Background(), req)
			if (err == nil) != (mode == "valid") || posts != 1 {
				t.Fatal(err, posts)
			}
		})
	}
}
