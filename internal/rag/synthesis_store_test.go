package rag

import (
	"errors"
	"strings"
	"testing"
)

func TestSynthesisCheckpointCrashRecoveryAndIdentity(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := synthesisTestDocument(t, s, strings.Repeat("Policy 17.25. 例外：七月。\n", 150))
	model := synthesisTestIdentity()
	r, err := s.CreateSynthesis("owner", "stable", "requirements", refs, model, 1024)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.CreateSynthesis("owner", "stable", "requirements", refs, model, 1024)
	if err != nil || replay.ID != r.ID {
		t.Fatal(replay, err)
	}
	changed := model
	changed.Language = "fr"
	if _, err = s.CreateSynthesis("owner", "stable", "requirements", refs, changed, 1024); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("model identity changed on retry", err)
	}
	r, lease, err := s.beginSynthesis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, b, ex := synthesisTestBatch(t, s, r)
	if b.EndOfUnit {
		t.Fatal("fixture did not span batches")
	}
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("duplicate publication", err)
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
	r, err = s.Synthesis("owner", r.ID)
	if err != nil || r.State != "interrupted" || r.CompletedBatches != 1 || r.NextByte != b.Primary.End || r.CompletedUnits != 0 {
		t.Fatal(r, err)
	}
	r, nextLease, err := s.beginSynthesis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, b, ex = synthesisTestBatch(t, s, r)
	if err = s.checkpointSynthesis(lease, 1, tr, b, ex); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("stale attempt published", err)
	}
	for r.UnitCursor < len(r.Plan.Units) {
		tr, b, ex = synthesisTestBatch(t, s, r)
		if err = s.checkpointSynthesis(nextLease, r.CompletedBatches, tr, b, ex); err != nil {
			t.Fatal(err)
		}
		r, err = s.Synthesis("owner", r.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.State != "awaiting_review" || !r.ReviewRequired || r.CompletedUnits != len(r.Plan.Units) {
		t.Fatal("candidate completion became approved answer", r)
	}
	page, err := s.SynthesisResults("owner", r.ID, 0, 1)
	if err != nil || len(page.Receipts) != 1 || page.NextOffset == nil {
		t.Fatal(page, err)
	}
	if _, err = s.SynthesisResults("outsider", r.ID, 0, 1); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("cross-user result", err)
	}
	if _, _, err = s.beginSynthesis("owner", r.ID); err == nil {
		t.Fatal("completed traversal restarted")
	}
}

func TestSynthesisCheckpointAtomicityAndTampering(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.CreateSynthesis("owner", "atomic", "summary", synthesisTestDocument(t, s, "Policy 17.25."), synthesisTestIdentity(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	r, lease, err := s.beginSynthesis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, b, ex := synthesisTestBatch(t, s, r)
	if _, err = s.db.Exec(`CREATE TRIGGER fail_synthesis_progress BEFORE UPDATE ON document_synthesis_configs BEGIN SELECT RAISE(ABORT,'simulated storage failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); err == nil {
		t.Fatal("storage failure ignored")
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_analysis_steps WHERE analysis_id=?`, r.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial checkpoint", count, err)
	}
	current, err := s.Synthesis("owner", r.ID)
	if err != nil || current.CompletedBatches != 0 || current.NextByte != r.NextByte {
		t.Fatal(current, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_synthesis_progress`); err != nil {
		t.Fatal(err)
	}
	ex.Receipt.Result.Candidates[0].Text = "forged prose"
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal("forged receipt accepted", err)
	}
	tr, b, ex = synthesisTestBatch(t, s, r)
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE document_analysis_steps SET result_json=replace(result_json,'Policy','Forged') WHERE analysis_id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SynthesisResults("owner", r.ID, 0, 1); !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal("persisted tampering disclosed", err)
	}
}
