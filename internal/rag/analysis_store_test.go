package rag

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func analysisDocuments(t *testing.T, s *SQLiteStore) []document.SourceRef {
	t.Helper()
	refs := []document.SourceRef{}
	for i, text := range []string{"Policy 17.25.\nException July.", "Policy 18.25.\nException August."} {
		parsed, err := NewDocumentParser().Parse([]byte(text), "policy.txt", ".txt")
		if err != nil {
			t.Fatal(err)
		}
		content, ranges, err := parsedSourceRanges(parsed)
		if err != nil {
			t.Fatal(err)
		}
		d := &Document{ID: []string{"before", "after"}[i], Name: "Policy", RawContent: content, Metadata: parsed.Metadata, SourceRanges: ranges, Extraction: parsed.Structure, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
			t.Fatal(err)
		}
		g, err := s.extraction(d.ID, d.ExtractionRevision)
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, document.SourceRef{DocumentID: d.ID, Revision: g.ID, OriginalSHA256: g.OriginalSHA256})
	}
	return refs
}

func TestAnalysisCheckpointRecoveryAndSourceRetention(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := analysisDocuments(t, s)
	r, err := s.CreateComparison("owner", "request", refs)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.CreateComparison("owner", "request", refs)
	if err != nil || replay.ID != r.ID {
		t.Fatal("idempotency lost", err)
	}
	if _, err = s.CreateComparison("owner", "request", []document.SourceRef{refs[1], refs[0]}); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("rebound request", err)
	}
	_, lease, err := s.beginAnalysis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.RecoverAnalyses(); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.Analysis("owner", r.ID)
	if err != nil || recovered.State != "interrupted" || recovered.CompletedSteps != 0 {
		t.Fatal(recovered, err)
	}
	graphs, err := s.analysisGraphs("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := document.CompareUnit(context.Background(), graphs[0], graphs[1], r.Pairs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(lease, 0, result); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("stale worker published", err)
	}
	_, newLease, err := s.beginAnalysis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(newLease, 0, result); err != nil {
		t.Fatal(err)
	}
	completed, err := s.Analysis("owner", r.ID)
	if err != nil || completed.CompletedUnits != 2 || completed.CompletedSteps != 1 || (completed.State != "complete" && completed.State != "incomplete") {
		t.Fatal(completed, err)
	}
	if err = s.checkpointComparison(newLease, 0, result); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("duplicate publication", err)
	}
	page, err := s.AnalysisResults("owner", r.ID, 0, 0, 1)
	if err != nil || len(page.Result.Changes) != 1 || page.NextOffset == nil {
		t.Fatal(page, err)
	}
	if _, err = s.AnalysisResults("outsider", r.ID, 0, 0, 100); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("cross-user result", err)
	}
	hold, err := s.SetDocumentRetention("owner", "before", 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("before"); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("hold failed to protect analysis sources", err)
	}
	if _, err = s.Analysis("owner", r.ID); err != nil {
		t.Fatal("held analysis disappeared", err)
	}
	if _, err = s.SetDocumentRetention("owner", "before", hold.Revision, 0, false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("before"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"document_analyses", "document_analysis_sources", "document_analysis_steps"} {
		var count int
		if err = s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("derived retention leak", table, count, err)
		}
	}
}

func TestAnalysisRevocationAndCancelledLease(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	refs := analysisDocuments(t, s)
	policy, err := s.CreateCollection("owner", "Private", []string{"reader"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", "before", policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateComparison("reader", "comparison", refs)
	if err != nil {
		t.Fatal(err)
	}
	_, lease, err := s.beginAnalysis("reader", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	graphs, err := s.analysisGraphs("reader", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := document.CompareUnit(context.Background(), graphs[0], graphs[1], r.Pairs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(lease, 0, result); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("revoked worker published", err)
	}
	if _, err = s.Analysis("reader", r.ID); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("revoked result disclosed", err)
	}
	if err = s.CancelAnalysis("reader", r.ID); err != nil {
		t.Fatal("owner cannot stop revoked job", err)
	}
	if _, err = s.CreateComparison("admin", "forbidden", refs); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("admin bypassed private content", err)
	}
	owned, err := s.CreateComparison("owner", "owned", refs)
	if err != nil {
		t.Fatal(err)
	}
	_, lease, err = s.beginAnalysis("owner", owned.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CancelAnalysis("owner", owned.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(lease, 0, result); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("cancelled worker published", err)
	}
}

func TestAnalysisEngineRunsWithoutEmbeddingOrGeneration(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateComparison("owner", "automatic", analysisDocuments(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = e.StartComparison("owner", r.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current, err := s.Analysis("owner", r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.State == "complete" || current.State == "incomplete" {
			if current.CompletedUnits != 2 {
				t.Fatal(current)
			}
			return
		}
		if current.State == "interrupted" {
			t.Fatal(current)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("analysis did not finish")
}
