package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestKnowledgeSynthesisReviewCommandRoundTrip(t *testing.T) {
	model := document.SynthesisModelIdentity{ModelID: "fixture", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "fixture", Revision: "1"}, Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}
	identity, _ := model.ID()
	text := "Policy"
	hash := sha256.Sum256([]byte(text))
	span := document.Span{Start: 0, End: len(text)}
	job := rag.SynthesisRecord{ID: "job", State: "awaiting_review", Model: model, PrimaryBytes: 1024, CompletedBatches: 1, CompletedUnits: 1, UnitCursor: 1, ReviewRequired: true, Plan: &document.AnalysisPlan{ID: "plan", Version: document.AnalysisVersion, Workflow: "summary", Sources: []document.SourceRef{{DocumentID: "source", Revision: "revision"}}, Units: []document.AnalysisUnit{{Ordinal: 0, Source: 0, UnitID: "unit", Span: span}}}}
	receipt := &document.SynthesisBatchReceipt{Method: "model", BatchID: "batch", ModelIdentity: identity, Primary: span, EndOfUnit: true, PromptTokens: 1000, ResponseSHA256: strings.Repeat("b", 64), Result: &document.SynthesisCandidates{BatchID: "batch", SemanticReviewRequired: true, Candidates: []document.SynthesisCandidate{{Text: text, Verification: "source_identity_only", Evidence: []document.SynthesisCitedQuote{{Text: text, Citation: document.AnalysisCitation{Source: 0, Revision: "revision", UnitID: "unit", Span: span, TextSHA256: hex.EncodeToString(hash[:])}}}}}}}
	page := rag.SynthesisResultPage{AnalysisID: "job", TotalBatches: 1, Receipts: []*document.SynthesisBatchReceipt{receipt}}
	review := rag.SynthesisReview{AnalysisID: "job", BatchID: "batch", ResponseSHA256: receipt.ResponseSHA256, Decisions: []rag.SynthesisDecision{{Decision: "unreviewed"}}}
	posts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/results") {
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/review") {
			if r.Method == "POST" {
				posts++
				var req rag.SynthesisReviewRequest
				if json.NewDecoder(r.Body).Decode(&req) != nil {
					w.WriteHeader(400)
					return
				}
				review.Revision++
				review.Decisions = req.Decisions
				review.SectionChecked = req.SectionChecked
				review.Author = "owner"
				review.UpdatedAt = "2026-11-13T12:00:00Z"
			}
			_ = json.NewEncoder(w).Encode(review)
			return
		}
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer s.Close()
	c, _ := serviceclient.New(s.URL, "key", s.Client())
	ctx := context.Background()
	if _, err := RunKnowledgeService(ctx, c, []string{"synthesis-review", "job", "0"}, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	req := rag.SynthesisReviewRequest{ID: "job", BatchID: "batch", ResponseSHA256: receipt.ResponseSHA256, Decisions: []rag.SynthesisDecision{{Decision: "keep", Note: "Checked source"}}, SectionChecked: true}
	raw, _ := json.Marshal(req)
	file := filepath.Join(t.TempDir(), "review.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	value, err := RunKnowledgeService(ctx, c, []string{"save-synthesis-review", file}, nil, nil, true)
	if err != nil || value.(rag.SynthesisReview).Revision != 1 || posts != 1 {
		t.Fatal(value, err, posts)
	}
	if err = os.WriteFile(file, []byte(`{"id":"job","author":"pretend-reviewer"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = RunKnowledgeService(ctx, c, []string{"save-synthesis-review", file}, nil, nil, true); err == nil || posts != 1 {
		t.Fatal("untrusted reviewer field submitted", err, posts)
	}
}
