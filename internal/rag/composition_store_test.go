package rag

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestCompositionCheckpointRecoveryAtomicityAndRetention(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	parent := completedCompositionParent(t, s, "owner")
	r, err := s.CreateComposition(ctx, "owner", "report", parent.ID, synthesisTestIdentity())
	if err != nil {
		t.Fatal(err)
	}
	r, lease, err := s.beginComposition(ctx, "owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := s.compositionTraversal(ctx, "owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := tr.Execute(ctx, tr.Next(), r.Model, &compositionFixtureProvider{}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER composition_fault BEFORE UPDATE OF completed_steps ON document_compositions BEGIN SELECT RAISE(ABORT,'fixture publication failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComposition(ctx, lease, tr, ex); err == nil {
		t.Fatal("publication fault ignored")
	}
	var count int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_composition_steps`).Scan(&count); err != nil || count != 0 || tr.Next().Ordinal != 0 {
		t.Fatal("orphan or advanced cursor", count, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER composition_fault`); err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComposition(ctx, lease, tr, ex); err != nil {
		t.Fatal(err)
	}
	if err = s.RecoverAnalyses(); err != nil {
		t.Fatal(err)
	}
	resumed, nextLease, err := s.beginComposition(ctx, "owner", r.ID)
	if err != nil || resumed.CompletedSteps != 1 {
		t.Fatal(resumed, err)
	}
	tr, err = s.compositionTraversal(ctx, "owner", r.ID)
	if err != nil || tr.Next().Ordinal != 1 {
		t.Fatal("replayed committed merge", err)
	}
	ex, err = tr.Execute(ctx, tr.Next(), r.Model, &compositionFixtureProvider{}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.checkpointComposition(ctx, lease, tr, ex); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("stale lease", err)
	}
	if err = s.checkpointComposition(ctx, nextLease, tr, ex); err != nil {
		t.Fatal(err)
	}
	hold, err := s.SetDocumentRetention("owner", "source", 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("source"); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("hold bypass", err)
	}
	if _, err = s.SetDocumentRetention("owner", "source", hold.Revision, 0, false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("source"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"document_compositions", "document_composition_steps"} {
		if err = s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("retention leak", table, count, err)
		}
	}
}

func TestCompositionMigrationPreservesParentAndTakesSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent := completedCompositionParent(t, s, "owner")
	if _, err = s.db.Exec(`DROP TABLE document_composition_steps; DROP TABLE document_compositions`); err != nil {
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
	r, err := s.CreateComposition(context.Background(), "owner", "report", parent.ID, synthesisTestIdentity())
	if err != nil || r.AnalysisID != parent.ID {
		t.Fatal(r, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
}
