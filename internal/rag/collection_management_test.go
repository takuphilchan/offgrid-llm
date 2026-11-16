package rag

import (
	"errors"
	"testing"
)

func TestCollectionReviewedMovementAndAuditAreAtomic(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.CreateCollection("owner", "Source", []string{"reader"}, []string{"editor"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateCollection("owner", "Destination", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AddDocument(&Document{ID: "private", RawContent: "Confidential content"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", "private", a.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.assignDocumentCollection("editor", "private", b.ID, 2, 1, 1); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("editor changed audience", err)
	}
	if _, err = s.UpdateCollection("owner", b.ID, 1, b.Name, []string{"new-reader"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.assignDocumentCollection("owner", "private", b.ID, 2, 1, 1); !errors.Is(err, ErrCollectionConflict) {
		t.Fatal("stale destination audience accepted", err)
	}
	if err = s.DocumentAccess("new-reader", "private", false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("failed move published source", err)
	}
	if _, err = s.assignDocumentCollection("owner", "private", b.ID, 2, 1, 2); err != nil {
		t.Fatal(err)
	}
	// Public list projections identify the binding, not the audience revision.
	// A forged input projection cannot assign a document to another audience.
	document, err := s.GetDocument("private")
	if err != nil || document.CollectionID != b.ID || document.CollectionRevision != 3 {
		t.Fatal("source projection did not follow movement", document, err)
	}
	if err = s.AddDocument(&Document{ID: "forged", RawContent: "Public input", CollectionID: b.ID, CollectionRevision: 99}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListDocuments()
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range listed {
		if doc.ID == "private" && (doc.CollectionID != b.ID || doc.CollectionRevision != 3) {
			t.Fatal("list returned stale binding", doc)
		}
		if doc.ID == "forged" && (doc.CollectionID != "workspace" || doc.CollectionRevision != 1) {
			t.Fatal("public fields changed authority", doc)
		}
	}
	if err = s.DocumentAccess("reader", "private", false); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("old membership still grants source", err)
	}
	if err = s.DocumentAccess("new-reader", "private", false); err != nil {
		t.Fatal(err)
	}
	events, err := s.CollectionAudit("owner", b.ID, 0)
	if err != nil || len(events) != 3 || events[2].Action != "document_moved" {
		t.Fatal(events, err)
	}
	if _, err = s.CollectionAudit("new-reader", b.ID, 0); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("audit owner boundary", err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER reject_collection_audit BEFORE INSERT ON knowledge_collection_audit BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateCollection("owner", b.ID, 2, "Should roll back", nil, nil); err == nil {
		t.Fatal("update survived lost audit")
	}
	p, err := s.Collection("new-reader", b.ID)
	if err != nil || p.Revision != 2 || p.Name != b.Name {
		t.Fatal("failed audit changed policy", p, err)
	}
}

func TestReviewedImportCannotPublishAfterAudienceChange(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AcceptImportIntoCollection("owner", "request", "source.txt", []byte("Sensitive source"), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := indexingImport(t, s, r, "owner")
	if _, err = s.UpdateCollection("owner", p.ID, 1, p.Name, []string{"new-reader"}, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.AddDocumentWithChunks(doc, nil, nil); err == nil {
		t.Fatal("unreviewed audience received imported source")
	}
	if got, _ := s.GetDocument(doc.ID); got != nil {
		t.Fatal("failed publication left document")
	}
	if _, err = s.Import("owner", r.ID); err != nil {
		t.Fatal("lost recovery read", err)
	}
	if err = s.stopImport("owner", r.ID, false); err != nil {
		t.Fatal("changed audience prevents cancellation", err)
	}
	if err = s.stopImport("owner", r.ID, true); err != nil {
		t.Fatal("changed audience prevents cleanup", err)
	}
}
