package rag

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestCollectionPolicyOwnershipRevisionAndRevocation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.CreateCollection("alice", "Clinic records", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor       string
		read, write bool
	}{
		{"alice", true, true}, {"reader", true, false}, {"editor", true, true}, {"stranger", false, false}, {"", false, false},
	} {
		if p.Allows(tc.actor, false) != tc.read || p.Allows(tc.actor, true) != tc.write {
			t.Fatal("wrong access", tc.actor)
		}
	}
	for _, actor := range []string{"reader", "editor", "stranger"} {
		if _, err := store.UpdateCollection(actor, p.ID, 1, "Forged", nil, nil); !errors.Is(err, ErrCollectionUnavailable) {
			t.Fatal("non-owner changed membership", actor, err)
		}
	}
	if _, err := store.Collection("stranger", p.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("private policy leaked", err)
	}
	updated, err := store.UpdateCollection("alice", p.ID, 1, "Clinic records", nil, []string{"editor"})
	if err != nil || updated.Revision != 2 || updated.Owner != "alice" {
		t.Fatal(updated, err)
	}
	if _, err := store.UpdateCollection("alice", p.ID, 1, "Stale edit", []string{"reader"}, nil); !errors.Is(err, ErrCollectionConflict) {
		t.Fatal("stale editor restored revoked access", err)
	}
	store.Close()
	store, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Collection("reader", p.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revocation lost on restart", err)
	}
	saved, err := store.Collection("editor", p.ID)
	if err != nil || !reflect.DeepEqual(saved, updated) {
		t.Fatal("saved policy changed", saved, err)
	}
	if _, err := store.UpdateCollection("alice", WorkspaceCollection, 1, "Make shared private", nil, nil); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("legacy workspace policy edited", err)
	}
}

func TestCollectionPolicyConcurrentEditsAndInvalidRecords(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	p, err := store.CreateCollection("owner", "Research", []string{"b", "a"}, nil)
	if err != nil || !reflect.DeepEqual(p.Readers, []string{"a", "b"}) {
		t.Fatal(p, err)
	}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"First", "Second"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := store.UpdateCollection("owner", p.ID, 1, name, nil, nil)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	passed, conflict := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else if errors.Is(err, ErrCollectionConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || conflict != 1 {
		t.Fatal("lost update", passed, conflict)
	}
	for _, members := range [][]string{{"same", "same"}, {""}, {" actor"}, {"bad\nactor"}, make([]string, 129)} {
		if _, err := store.CreateCollection("owner", "Bad", members, nil); err == nil {
			t.Fatal("invalid membership accepted")
		}
	}
	if _, err := store.db.Exec(`UPDATE knowledge_collections SET schema_version=99 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Collection("owner", p.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("future policy authorized", err)
	}
	if _, err := store.UpdateCollection("owner", p.ID, 2, "Downgrade", nil, nil); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("future policy downgraded", err)
	}
	if _, err := store.db.Exec(`UPDATE knowledge_collections SET schema_version=1,readers='["bad","bad"]' WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Collection("owner", p.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("corrupt policy authorized", err)
	}
}

func TestCollectionPolicyMigrationPreservesLegacySourcesAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			store.Close()
		}
	}()
	doc := &Document{ID: "legacy", Name: "Existing notes", RawContent: "Preserved original extraction", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	// Recreate the pre-policy database shape, retaining its real document table.
	if _, err := store.db.Exec(`DROP TRIGGER documents_retention_insert; DROP TABLE document_retention; DROP TRIGGER imports_collection_insert; DROP TABLE import_collections; DROP TRIGGER documents_collection_insert; DROP TABLE document_collections; DROP TABLE knowledge_collections; DROP TABLE knowledge_policy_migrations;`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.GetDocument(doc.ID)
	if err != nil || saved.RawContent != doc.RawContent {
		t.Fatal("migration changed document", saved, err)
	}
	shared, err := store.Collection("existing-user", WorkspaceCollection)
	if err != nil || shared.Owner != "" || !shared.Allows("existing-user", true) {
		t.Fatal("legacy policy changed", shared, err)
	}
	// Permission to use Knowledge is still required by the service; the policy
	// layer never recreates a missing shared grant on the next startup.
	if err := store.DocumentAccess("existing-user", doc.ID, false); err != nil {
		t.Fatal("legacy document lost its workspace binding", err)
	}
	if _, err := store.db.Exec(`DELETE FROM document_collections; DELETE FROM knowledge_collections WHERE id='workspace'`); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if reopened, err := NewSQLiteStore(dir); err == nil {
		reopened.Close()
		t.Fatal("missing policy silently regranted")
	}
	db, err := storage.OpenSQLite(filepath.Join(dir, "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var text string
	if err := db.QueryRow(`SELECT raw_content FROM documents WHERE id='legacy'`).Scan(&text); err != nil || text != doc.RawContent {
		t.Fatal("failed migration lost source", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_policy_migrations(version) VALUES(99)`); err != nil {
		t.Fatal(err)
	}
	if reopened, err := NewSQLiteStore(dir); err == nil {
		reopened.Close()
		t.Fatal("future schema was downgraded")
	} else if !strings.Contains(err.Error(), "newer") {
		t.Fatal(err)
	}
}
