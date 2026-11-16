package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// Invoked only by the opt-in real-runtime test with explicit existing assets.
func runRealComposition(t *testing.T, ctx context.Context, s *Server, store *rag.SQLiteStore, middleware *users.Middleware, key, owner string, parent rag.SynthesisRecord) {
	t.Helper()
	started := time.Now()
	body, _ := json.Marshal(compositionCreateRequest{RequestKey: "compose-" + parent.Model.Language, AnalysisID: parent.ID, Model: parent.Model.ModelID, Language: parent.Model.Language})
	r := httptest.NewRequest("POST", "/v1/document-analyses/synthesis/compositions", bytes.NewReader(body)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(s.handleDocumentCompositions))).ServeHTTP(w, r)
	var job rag.CompositionStatus
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &job) != nil {
		t.Fatal("composition create", w.Code, w.Body.String())
	}
	for job.State == "queued" || job.State == "running" {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
		current, err := store.Composition(ctx, owner, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		job = current.Status()
	}
	if job.State != "awaiting_review" {
		t.Fatal("composition incomplete", job.State, job.ErrorCode)
	}
	report, err := store.CompositionReportView(ctx, owner, job.ID)
	if err != nil || len(report.Sections) == 0 || !report.SemanticReviewRequired {
		t.Fatal("no validated composition", err, report)
	}
	for i, section := range report.Sections {
		evidence, err := store.CompositionEvidence(ctx, owner, job.ID, i, 0)
		if err != nil || len(evidence.Items) == 0 || section.Verification != "source_identity_only" {
			t.Fatal("missing original evidence", err)
		}
		t.Logf("composition section language=%s text=%s", parent.Model.Language, section.Text)
	}
	artifact, err := store.ExportComposition(ctx, owner, job.ID, "json")
	if err != nil || !strings.Contains(artifact.Content, "semantic_review_required") {
		t.Fatal("composition export", err)
	}
	t.Logf("composition steps=%d sections=%d gaps=%v extraction_gaps=%v elapsed=%s", job.CompletedSteps, len(report.Sections), report.Gaps, report.ExtractionGaps, time.Since(started))
}
