package rag

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func policyDocument(t *testing.T, store *SQLiteStore, id string) *Document {
	t.Helper()
	doc := &Document{ID: id, Name: id, RawContent: "Source " + id, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestDocumentPolicyBindingsRevocationAndRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	policyDocument(t, store, "shared")
	private := policyDocument(t, store, "private")
	p, err := store.CreateCollection("owner", "Private records", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	if revision, err := store.AssignDocumentCollection("owner", private.ID, p.ID, 1); err != nil || revision != 2 {
		t.Fatal(revision, err)
	}
	for _, tc := range []struct {
		actor          string
		write, allowed bool
	}{
		{"owner", true, true}, {"reader", false, true}, {"reader", true, false},
		{"editor", true, true}, {"stranger", false, false}, {"", false, false},
	} {
		err := store.DocumentAccess(tc.actor, private.ID, tc.write)
		if (err == nil) != tc.allowed {
			t.Fatal("incorrect access", tc, err)
		}
	}
	if _, err := store.AssignDocumentCollection("editor", private.ID, WorkspaceCollection, 2); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("editor changed audience", err)
	}
	if _, err := store.AssignDocumentCollection("owner", private.ID, WorkspaceCollection, 1); !errors.Is(err, ErrCollectionConflict) {
		t.Fatal("stale move accepted", err)
	}
	for _, tc := range []struct {
		actor           string
		requested, want []string
		denied          bool
	}{
		{"reader", nil, []string{"private", "shared"}, false},
		{"stranger", nil, []string{"shared"}, false},
		{"reader", []string{}, []string{}, false},
		{"stranger", []string{"private"}, nil, true},
		{"reader", []string{"shared", "missing"}, nil, true},
		{"reader", []string{"private", "private"}, nil, true},
	} {
		ids, err := store.AuthorizedDocumentIDs(tc.actor, tc.requested)
		if tc.denied {
			if !errors.Is(err, ErrCollectionUnavailable) {
				t.Fatal(tc, ids, err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(ids, tc.want) {
			t.Fatal(tc, ids, err)
		}
	}
	// A legacy replacement cannot reset a private binding through its insert
	// trigger. Normal atomic reindexing must retain that same binding.
	if err := store.AddDocument(private); err == nil {
		t.Fatal("legacy replacement reset private policy")
	}
	if err := store.ReplaceDocumentWithChunks(private, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.DocumentAccess("stranger", private.ID, false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("reindex shared private document", err)
	}
	if _, err := store.UpdateCollection("owner", p.ID, p.Revision, p.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.DocumentAccess("reader", private.ID, false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("revoked reader retained access", err)
	}
	store.Close()
	store, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := store.AuthorizedDocumentIDs("reader", nil)
	if err != nil || !reflect.DeepEqual(ids, []string{"shared"}) {
		t.Fatal("revocation lost on restart", ids, err)
	}
	if err := store.DeleteDocument(private.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM document_collections WHERE document_id='private'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("deleted binding retained", count, err)
	}
	if err := store.DocumentAccess("reader", "shared", true); err != nil {
		t.Fatal("unrelated document changed", err)
	}
}

func TestDocumentPolicyCorruptBindingsNeverBecomeShared(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE document_collections SET schema_version=99 WHERE document_id='doc'`,
		`UPDATE document_collections SET revision=0 WHERE document_id='doc'`,
		`DELETE FROM document_collections WHERE document_id='doc'`,
	} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			store, err := NewSQLiteStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { store.Close() }()
			doc := policyDocument(t, store, "doc")
			if _, err := store.db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			store.Close()
			store, err = NewSQLiteStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.DocumentAccess("owner", doc.ID, false); !errors.Is(err, ErrCollectionUnavailable) {
				t.Fatal("corrupt binding authorized", err)
			}
			if err := store.AddDocument(doc); err == nil {
				t.Fatal("legacy replacement restored access")
			}
			ids, err := store.AuthorizedDocumentIDs("owner", nil)
			if err != nil || len(ids) != 0 {
				t.Fatal("corrupt binding selected", ids, err)
			}
			saved, err := store.GetDocument(doc.ID)
			if err != nil || saved.RawContent != doc.RawContent {
				t.Fatal("source lost", err)
			}
		})
	}
}
