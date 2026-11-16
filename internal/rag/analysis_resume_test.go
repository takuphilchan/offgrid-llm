package rag

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestAnalysisResumesCommittedUnitsAndRollsBackPublication(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	refs := []document.SourceRef{}
	for i, text := range []string{"A1\nB1", "A2\nB2"} {
		g := &document.Extraction{SourceKind: "legacy_text", Format: "txt", Method: "native", Text: text, Coverage: &document.Coverage{Status: "complete", TotalUnits: 2, ReadableUnits: 2}, Units: []document.Unit{{ID: "first", Kind: "section", Index: 1, State: "ready", Span: document.Span{End: 3}}, {ID: "second", Kind: "section", Index: 2, State: "ready", Span: document.Span{Start: 3, End: 5}}}, Blocks: []document.Block{{ID: "a", UnitID: "first", Kind: "paragraph", Method: "native", Span: document.Span{End: 3}}, {ID: "b", UnitID: "second", Kind: "paragraph", Method: "native", Span: document.Span{Start: 3, End: 5}}}}
		if err = g.Seal(); err != nil {
			t.Fatal(err)
		}
		d := &Document{ID: []string{"a", "b"}[i], Name: "Policy", RawContent: text, Extraction: g, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, document.SourceRef{DocumentID: d.ID, Revision: d.ExtractionRevision})
	}
	r, err := s.CreateComparison("owner", "resume", refs)
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
	first, err := document.CompareUnit(context.Background(), graphs[0], graphs[1], r.Pairs[0])
	if err != nil {
		t.Fatal(err)
	}
	// Preserve a review flag even when retry clears the transient error code.
	first.AmbiguousCells = 1
	if err = s.checkpointComparison(lease, 0, first); err != nil {
		t.Fatal(err)
	}
	if err = s.RecoverAnalyses(); err != nil {
		t.Fatal(err)
	}
	resumed, newLease, err := s.beginAnalysis("owner", r.ID)
	if err != nil || resumed.CompletedSteps != 1 || resumed.CompletedUnits != 2 {
		t.Fatal(resumed, err)
	}
	second, err := document.CompareUnit(context.Background(), graphs[0], graphs[1], r.Pairs[1])
	if err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(lease, 1, second); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("stale attempt committed", err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER analysis_fault BEFORE UPDATE OF completed_steps ON document_analyses BEGIN SELECT RAISE(ABORT,'fixture checkpoint failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(newLease, 1, second); err == nil {
		t.Fatal("publication fault ignored")
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_analysis_steps WHERE analysis_id=?`, r.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("orphan checkpoint", count, err)
	}
	after, err := s.Analysis("owner", r.ID)
	if err != nil || after.CompletedSteps != 1 {
		t.Fatal("failed checkpoint advanced progress", after, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER analysis_fault`); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComparison(newLease, 1, second); err != nil {
		t.Fatal(err)
	}
	after, err = s.Analysis("owner", r.ID)
	if err != nil || after.State != "incomplete" || !after.ReviewRequired || after.CompletedSteps != 2 || after.CompletedUnits != 4 {
		t.Fatal("review gap lost on resume", after, err)
	}
	page, err := s.AnalysisResults("owner", r.ID, 0, 0, 100)
	if err != nil || page.Result.AmbiguousCells != 1 {
		t.Fatal("completed checkpoint was replayed", page, err)
	}
	if _, err = s.ReadAnalysisSource("owner", r.ID, 0, 0, "before"); err != nil {
		t.Fatal(err)
	}
	// Corrupt references must fail at readback even though they came from disk.
	if _, err = s.db.Exec(`UPDATE document_analysis_steps SET result_json=replace(result_json,?,?) WHERE analysis_id=? AND ordinal=0`, first.Changes[0].Before.TextSHA256, "forged", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadAnalysisSource("owner", r.ID, 0, 0, "before"); !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal("forged retained citation opened", err)
	}
}
