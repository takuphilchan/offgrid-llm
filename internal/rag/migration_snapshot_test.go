package rag

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestDocumentUpgradeSnapshotRetainsContentAndRowIdentity(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	now := time.Now().UTC()
	doc := &Document{ID: "legacy", Name: "Source", RawContent: "Critical amount 17.25", CreatedAt: now, UpdatedAt: now}
	if err = s.AddDocumentWithChunks(doc, []*Chunk{{ID: "old-chunk", DocumentID: doc.ID, Content: doc.RawContent, CreatedAt: now}}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	// Remove the additive projection to simulate the pre-upgrade schema.
	if _, err = s.db.Exec(`DROP TRIGGER embedding_projection_invalidate; ALTER TABLE chunks DROP COLUMN embedding_v1`); err != nil {
		t.Fatal(err)
	}
	var rowid int64
	if err = s.db.QueryRow(`SELECT rowid FROM chunks WHERE id='old-chunk'`).Scan(&rowid); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups, err := filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
	copyDB, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var actual int64
	var text string
	var binaryColumns int
	if err = copyDB.QueryRow(`SELECT rowid,content FROM chunks WHERE id='old-chunk'`).Scan(&actual, &text); err != nil || actual != rowid || text != doc.RawContent {
		t.Fatal("backup changed source/row identity", actual, text, err)
	}
	if err = copyDB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chunks') WHERE name='embedding_v1'`).Scan(&binaryColumns); err != nil || binaryColumns != 0 {
		t.Fatal("backup contains migrated schema", err)
	}
	if got, err := s.GetDocument("legacy"); err != nil || got.RawContent != doc.RawContent {
		t.Fatal("upgrade changed source", err)
	}
	// Opening the upgraded database does not create another private copy.
	second, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
	backups, _ = filepath.Glob(filepath.Join(dir, ".before-document-upgrade-*.db"))
	if len(backups) != 1 {
		t.Fatal("duplicate snapshot")
	}
}
