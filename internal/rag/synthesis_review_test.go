package rag

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSynthesisReviewMigrationPreservesCommittedEvidence(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := completedSynthesisExport(t, s)
	if _, err = s.db.Exec(`DROP TABLE document_synthesis_reviews`); err != nil {
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
	v, err := s.SynthesisReview(context.Background(), "owner", r.ID, 0)
	if err != nil || v.Revision != 0 {
		t.Fatal(v, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	current, err := s.Synthesis("owner", r.ID)
	if err != nil || current.Plan.ID != r.Plan.ID || current.CompletedBatches != r.CompletedBatches {
		t.Fatal("migration changed evidence", current, err)
	}
}

func TestSynthesisReviewRevisionsAccessExportAndRetention(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := completedSynthesisExport(t, s)
	ctx := context.Background()
	v, err := s.SynthesisReview(ctx, "owner", r.ID, 0)
	if err != nil || v.Revision != 0 || len(v.Decisions) != 1 || v.Decisions[0].Decision != "unreviewed" {
		t.Fatal(v, err)
	}
	req := SynthesisReviewRequest{ID: r.ID, Batch: 0, BatchID: v.BatchID, ResponseSHA256: v.ResponseSHA256, Decisions: []SynthesisDecision{{Decision: "keep", Note: "Verified exception against source."}}, SectionChecked: true}
	saved, err := s.SaveSynthesisReview(ctx, "owner", req)
	if err != nil || saved.Revision != 1 || saved.Author != "owner" || saved.UpdatedAt == "" {
		t.Fatal(saved, err)
	}
	if _, err = s.SaveSynthesisReview(ctx, "owner", req); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("stale write", err)
	}
	if _, err = s.SynthesisReview(ctx, "other", r.ID, 0); err == nil {
		t.Fatal("private review leaked")
	}
	if _, err = s.SaveSynthesisReview(ctx, "other", req); err == nil {
		t.Fatal("private review changed")
	}
	for _, format := range []string{"json", "csv", "markdown"} {
		a, e := s.ExportSynthesis(ctx, "owner", r.ID, format)
		if e != nil || !strings.Contains(a.Content, "section_review") || !strings.Contains(a.Content, "Verified exception") || !strings.Contains(a.Content, "source_identity_only") {
			t.Fatal(format, a, e)
		}
	}
	req.ExpectedRevision = 1
	req.SectionChecked = false
	req.Decisions[0].Decision = "exclude"
	if _, err = s.SaveSynthesisReview(ctx, "owner", req); err != nil {
		t.Fatal(err)
	}
	old, e := s.SynthesisReview(ctx, "owner", r.ID, 0, 1)
	if e != nil || old.Decisions[0].Decision != "keep" || !old.SectionChecked {
		t.Fatal("review history changed", old, e)
	}
	if _, e = s.SynthesisReview(ctx, "owner", r.ID, 0, 3); !errors.Is(e, ErrAnalysisUnavailable) {
		t.Fatal("invented review revision", e)
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_synthesis_reviews WHERE analysis_id=?`, r.ID).Scan(&count); err != nil || count != 2 {
		t.Fatal("history lost", count, err)
	}
	current, _ := s.Synthesis("owner", r.ID)
	if current.State != "awaiting_review" || !current.ReviewRequired {
		t.Fatal("human review promoted model guarantee")
	}
	hold, err := s.SetDocumentRetention("owner", "source", 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("source"); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("owner", "source", hold.Revision, 0, false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("source"); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_synthesis_reviews`).Scan(&count); err != nil || count != 0 {
		t.Fatal("review retention leak", count, err)
	}
}

func TestSynthesisReviewRejectsInvalidAndConcurrentWrites(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := completedSynthesisExport(t, s)
	ctx := context.Background()
	v, _ := s.SynthesisReview(ctx, "owner", r.ID, 0)
	base := SynthesisReviewRequest{ID: r.ID, Batch: 0, BatchID: v.BatchID, ResponseSHA256: v.ResponseSHA256, Decisions: []SynthesisDecision{{Decision: "keep"}}}
	for _, mutate := range []func(*SynthesisReviewRequest){
		func(q *SynthesisReviewRequest) { q.BatchID = "wrong" }, func(q *SynthesisReviewRequest) { q.ResponseSHA256 = strings.Repeat("0", 64) },
		func(q *SynthesisReviewRequest) { q.Decisions = nil }, func(q *SynthesisReviewRequest) { q.Decisions = []SynthesisDecision{{Decision: "approved"}} },
		func(q *SynthesisReviewRequest) {
			q.Decisions = []SynthesisDecision{{Decision: "unreviewed"}}
			q.SectionChecked = true
		},
		func(q *SynthesisReviewRequest) {
			q.Decisions = []SynthesisDecision{{Decision: "keep", Note: strings.Repeat("x", 2049)}}
		},
	} {
		q := base
		mutate(&q)
		if _, err = s.SaveSynthesisReview(ctx, "owner", q); !errors.Is(err, ErrAnalysisInvalid) {
			t.Fatal("accepted invalid", q, err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.SaveSynthesisReview(ctx, "owner", base); results <- e }()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrAnalysisConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	base.ExpectedRevision = 1
	if _, err = s.db.Exec(`UPDATE document_synthesis_reviews SET storage_bytes=?`, 8<<20); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSynthesisReview(ctx, "owner", base); !errors.Is(err, ErrAnalysisLimit) {
		t.Fatal("review quota bypassed", err)
	}
	if _, err = s.db.Exec(`UPDATE document_synthesis_reviews SET review_json=replace(review_json,'"revision":1','"revision":9')`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportSynthesis(ctx, "owner", r.ID, "json"); !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal("tampered review exported", err)
	}
}
