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
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestCompositionClientValidatesReportAndOriginalEvidence(t *testing.T) {
	for _, mode := range []string{"valid", "wrong_plan", "approved", "wrong_revision", "wrong_quote", "candidate_digest", "excluded", "unchecked_keep", "bad_page"} {
		t.Run(mode, func(t *testing.T) {
			parent, page := synthesisClientFixture()
			parent.Plan.Workflow = "summary"
			candidate := page.Receipts[0].Result.Candidates[0]
			candidate.Conditions = []document.SynthesisCitedQuote{}
			candidate.Exceptions = []document.SynthesisCitedQuote{}
			data, _ := json.Marshal(candidate)
			sum := sha256.Sum256(data)
			job := rag.CompositionStatus{ID: "composition", AnalysisID: parent.ID, PlanID: "tree", SourcePlanID: parent.Plan.ID, Model: parent.Model, State: "awaiting_review", Attempt: 1, CompletedSteps: 1, TotalSteps: 1, ReviewsCurrent: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			report := rag.CompositionReportView{Composition: job, SourceUnits: len(parent.Plan.Units), Sections: []rag.CompositionSectionView{{Text: candidate.Text, Contributors: 1, Verification: "source_identity_only"}}, Gaps: []rag.CompositionGapCount{}, ExtractionGaps: []rag.CompositionGapCount{}, CandidateGaps: []rag.CompositionGapCount{}, SemanticReviewRequired: true}
			leaf := document.CompositionLeaf{ID: "leaf", BatchID: page.Receipts[0].BatchID, CandidateSHA256: hex.EncodeToString(sum[:]), ReviewRevision: 1, Decision: "keep", SectionChecked: true}
			evidence := rag.CompositionEvidencePage{CompositionID: job.ID, PlanID: job.PlanID, Total: 1, Items: []rag.CompositionEvidence{{Input: leaf, Candidate: candidate}}}
			switch mode {
			case "wrong_plan":
				report.Composition.SourcePlanID = "other"
			case "approved":
				report.SemanticReviewRequired = false
			case "wrong_revision":
				evidence.Items[0].Candidate.Evidence[0].Citation.Revision = "other"
			case "wrong_quote":
				evidence.Items[0].Candidate.Evidence[0].Text = "forged"
			case "candidate_digest":
				evidence.Items[0].Input.CandidateSHA256 = strings.Repeat("a", 64)
			case "excluded":
				evidence.Items[0].Input.Decision = "exclude"
			case "unchecked_keep":
				evidence.Items[0].Input.Decision = "unreviewed"
			case "bad_page":
				v := 8
				evidence.NextOffset = &v
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var value any = job
				switch {
				case strings.HasSuffix(r.URL.Path, "/report"):
					value = report
				case strings.HasSuffix(r.URL.Path, "/evidence"):
					value = evidence
				case r.URL.Path == "/v1/document-analyses/synthesis":
					value = parent
				}
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer s.Close()
			c, _ := New(s.URL, "key", s.Client())
			_, err := c.DocumentCompositionEvidence(context.Background(), job.ID, 0, 0)
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}

func TestCompositionClientDoesNotReplayUncertainCreation(t *testing.T) {
	posts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(503) }))
	defer s.Close()
	c, _ := New(s.URL, "key", s.Client())
	_, err := c.CreateDocumentComposition(context.Background(), DocumentCompositionRequest{RequestKey: "key", AnalysisID: "parent", Model: "local", Language: "en"})
	if err == nil || posts != 1 {
		t.Fatal(err, posts)
	}
}
