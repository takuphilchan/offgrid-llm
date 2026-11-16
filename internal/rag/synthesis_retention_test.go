package rag

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSynthesisRetentionAndSharedRequestKeys(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	refs := analysisDocuments(t, s)
	r, err := s.CreateSynthesis("owner", "shared", "summary", refs, synthesisTestIdentity(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateComparison("owner", "shared", refs); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("key crossed workflows", err)
	}
	if _, err = s.CreateComparison("owner", "comparison", refs); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateSynthesis("owner", "comparison", "summary", refs, synthesisTestIdentity(), 1024); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("key crossed workflows", err)
	}
	r, lease, err := s.beginSynthesis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, b, ex := synthesisTestBatch(t, s, r)
	if err = s.checkpointSynthesis(lease, 0, tr, b, ex); err != nil {
		t.Fatal(err)
	}
	hold, err := s.SetDocumentRetention("owner", "before", 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("before"); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal("hold bypassed", err)
	}
	if _, err = s.SynthesisResults("owner", r.ID, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("owner", "before", hold.Revision, 0, false); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteDocument("before"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"document_analyses", "document_analysis_steps", "document_analysis_sources", "document_synthesis_configs"} {
		var count int
		if err = s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("retention leak", table, count, err)
		}
	}
}

func TestSynthesisMigrationPreservesComparisonAndCreatesSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateComparison("owner", "old", analysisDocuments(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`DROP TABLE document_synthesis_configs`); err != nil {
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
	current, err := s.Analysis("owner", r.ID)
	if err != nil || current.Plan.ID != r.Plan.ID {
		t.Fatal("comparison changed", current, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	backup, err := sql.Open("sqlite", files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var plan string
	if err = backup.QueryRow(`SELECT plan_json FROM document_analyses WHERE id=? AND owner='owner'`, r.ID).Scan(&plan); err != nil || plan == "" {
		t.Fatal("snapshot lost job", err)
	}
	var configs int
	if err = backup.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='document_synthesis_configs'`).Scan(&configs); err != nil || configs != 0 {
		t.Fatal("snapshot taken after migration", configs, err)
	}
}
