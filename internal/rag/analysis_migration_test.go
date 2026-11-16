package rag

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestAnalysisMigrationBacksUpExistingSourceIdentities(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := analysisDocuments(t, s)
	p, err := s.CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", "before", p.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`DROP TRIGGER document_analysis_source_delete`, `DROP TABLE document_analysis_steps`, `DROP TABLE document_analysis_sources`, `DROP TABLE document_analyses`} {
		if _, err = s.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	files, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatal("missing migration snapshot", files, err)
	}
	backup, err := sql.Open("sqlite", files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	for _, ref := range refs {
		var revision string
		if err = backup.QueryRow(`SELECT extraction_revision FROM documents WHERE id=?`, ref.DocumentID).Scan(&revision); err != nil || revision != ref.Revision {
			t.Fatal("snapshot changed source identity", revision, err)
		}
		current, err := s.GetDocument(ref.DocumentID)
		if err != nil || current.ExtractionRevision != revision {
			t.Fatal("migration changed source", err)
		}
	}
	var collection string
	if err = backup.QueryRow(`SELECT collection_id FROM document_collections WHERE document_id='before'`).Scan(&collection); err != nil || collection != p.ID {
		t.Fatal("snapshot lost audience", collection, err)
	}
	if err = s.DocumentAccess("outsider", "before", false); err == nil {
		t.Fatal("migration broadened audience")
	}
	if _, err = s.CreateComparison("owner", "migrated", refs); err != nil {
		t.Fatal(err)
	}
}
