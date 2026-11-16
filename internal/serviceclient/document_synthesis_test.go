package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func synthesisClientFixture() (rag.SynthesisRecord, rag.SynthesisResultPage) {
	text := "政策 17.25"
	hash := sha256.Sum256([]byte(text))
	span := document.Span{Start: 0, End: len(text)}
	model := document.SynthesisModelIdentity{ModelID: "fixture", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "fixture", Revision: "1"}, Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}
	id, _ := model.ID()
	job := rag.SynthesisRecord{ID: "job", State: "awaiting_review", Model: model, PrimaryBytes: 1024, CompletedBatches: 1, CompletedUnits: 1, UnitCursor: 1, ReviewRequired: true, Plan: &document.AnalysisPlan{ID: "plan", Version: document.AnalysisVersion, Workflow: "requirements", Sources: []document.SourceRef{{DocumentID: "source", Revision: "revision"}}, Units: []document.AnalysisUnit{{Ordinal: 0, Source: 0, UnitID: "unit", Span: span}}}}
	quote := document.SynthesisCitedQuote{Text: text, Citation: document.AnalysisCitation{Source: 0, Revision: "revision", UnitID: "unit", Span: span, TextSHA256: hex.EncodeToString(hash[:])}}
	page := rag.SynthesisResultPage{AnalysisID: "job", TotalBatches: 1, Receipts: []*document.SynthesisBatchReceipt{{Method: "model", BatchID: "batch", ModelIdentity: id, UnitOrdinal: 0, Primary: span, EndOfUnit: true, PromptTokens: 1000, ResponseSHA256: strings.Repeat("b", 64), Result: &document.SynthesisCandidates{BatchID: "batch", SemanticReviewRequired: true, Candidates: []document.SynthesisCandidate{{Text: text, Verification: "source_identity_only", Evidence: []document.SynthesisCitedQuote{quote}, Conditions: []document.SynthesisCitedQuote{}, Exceptions: []document.SynthesisCitedQuote{}}}}}}}
	return job, page
}

func TestSynthesisClientRejectsForgedEvidenceAndIdentity(t *testing.T) {
	for _, mode := range []string{"valid", "growing", "quote", "model", "method", "range", "empty", "unmeasured", "pagination", "approved"} {
		t.Run(mode, func(t *testing.T) {
			job, page := synthesisClientFixture()
			switch mode {
			case "growing":
				job.State = "running"
				job.CompletedBatches = 0
				job.CompletedUnits = 0
				job.UnitCursor = 0
			case "quote":
				page.Receipts[0].Result.Candidates[0].Evidence[0].Text = "forged"
			case "model":
				page.Receipts[0].ModelIdentity = "other"
			case "method":
				page.Receipts[0].Method = "approved"
			case "range":
				page.Receipts[0].Primary.End++
			case "empty":
				page.Receipts[0].Method = "empty_source"
			case "unmeasured":
				page.Receipts[0].PromptTokens = 0
			case "pagination":
				next := 7
				page.NextOffset = &next
			case "approved":
				page.Receipts[0].Result.SemanticReviewRequired = false
			}
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if strings.HasSuffix(r.URL.Path, "/results") {
					_ = json.NewEncoder(w).Encode(page)
				} else {
					_ = json.NewEncoder(w).Encode(job)
				}
			}))
			defer s.Close()
			client, err := New(s.URL, "key", s.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.DocumentSynthesisResults(context.Background(), "job", 0)
			valid := mode == "valid" || mode == "growing"
			if (err == nil) != valid || calls != 2 {
				t.Fatal(mode, err, calls)
			}
		})
	}
}

func TestSynthesisCreationNeverReplaysUnconfirmedResponse(t *testing.T) {
	job, _ := synthesisClientFixture()
	job.Model.Language = "fr"
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _ = json.NewEncoder(w).Encode(job) }))
	defer s.Close()
	c, err := New(s.URL, "key", s.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateDocumentSynthesis(context.Background(), DocumentSynthesisRequest{RequestKey: "request", Workflow: "requirements", Sources: job.Plan.Sources, Model: "fixture", Language: "en"})
	if err == nil || calls != 1 {
		t.Fatal("unconfirmed create replay", err, calls)
	}
}
