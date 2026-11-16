package rag

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDocumentLifecycleAuditIsAtomicAndContentFree(t *testing.T) {
	e, doc, policy := correctionFixture(t)
	s := e.store.(*SQLiteStore)
	if _, err := s.SetDocumentRetention("owner", doc.ID, 1, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := e.ForActor("owner").RemoveDocument(doc.ID); !errors.Is(err, ErrDocumentHeld) {
		t.Fatal(err)
	}
	if _, err := s.SetDocumentRetention("owner", doc.ID, 2, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := e.ForActor("reader").RemoveDocument(doc.ID); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal(err)
	}
	if err := e.ForActor("owner").RemoveDocument(doc.ID); err != nil {
		t.Fatal(err)
	}
	events, err := s.CollectionAudit("owner", policy.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"legal_hold_added": 1, "legal_hold_released": 1, "document_deleted": 1}
	for _, event := range events {
		if _, ok := want[event.Action]; !ok {
			continue
		}
		want[event.Action]--
		if event.Actor != "owner" || event.Subject != doc.ID || event.CreatedAt.IsZero() {
			t.Fatal(event)
		}
		if strings.Contains(event.Subject, "clause") {
			t.Fatal("document body entered audit")
		}
	}
	for action, count := range want {
		if count != 0 {
			t.Fatalf("%s: remaining %d", action, count)
		}
	}
	if _, err := s.CollectionAudit("reader", policy.ID, 0); !errors.Is(err, ErrCollectionUnavailable) {
		t.Fatal("reader obtained owner audit", err)
	}
}

func TestDocumentAuditFailureRollsBackHoldAndDeletion(t *testing.T) {
	e, doc, _ := correctionFixture(t)
	s := e.store.(*SQLiteStore)
	_, err := s.db.Exec(`CREATE TRIGGER reject_document_audit BEFORE INSERT ON knowledge_collection_audit BEGIN SELECT RAISE(ABORT,'injected audit failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("owner", doc.ID, 1, 0, true); err == nil {
		t.Fatal("hold succeeded without audit")
	}
	retention, err := e.ForActor("owner").Retention(doc.ID)
	if err != nil || retention.Revision != 1 || retention.LegalHold {
		t.Fatal("failed hold mutated policy", retention, err)
	}
	if err = e.ForActor("owner").RemoveDocument(doc.ID); err == nil {
		t.Fatal("deletion succeeded without audit")
	}
	kept, err := s.GetDocument(doc.ID)
	if err != nil || kept == nil || kept.ExtractionRevision != doc.ExtractionRevision {
		t.Fatal("failed deletion removed source", err)
	}
}

func TestCorrectionAuditFailureRetainsCurrentEvidence(t *testing.T) {
	e, doc, policy := correctionFixture(t)
	s := e.store.(*SQLiteStore)
	embed := func(_ context.Context, texts []string) ([][]float32, error) {
		vectors := make([][]float32, len(texts))
		for i := range vectors {
			vectors[i] = []float32{0, 1}
		}
		return vectors, nil
	}
	_, err := s.db.Exec(`CREATE TRIGGER reject_correction_audit BEFORE INSERT ON knowledge_collection_audit WHEN NEW.action='extraction_corrected' BEGIN SELECT RAISE(ABORT,'injected audit failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	old := doc.ExtractionRevision
	if _, err = e.correctDocument(e.ForActor("editor").context(context.Background()), doc.ID, old, correctionFor(doc), embed); err == nil {
		t.Fatal("correction succeeded without audit")
	}
	current, err := s.GetDocument(doc.ID)
	if err != nil || current.ExtractionRevision != old || !strings.Contains(current.RawContent, "17.25") {
		t.Fatal("failed correction replaced current evidence", err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER reject_correction_audit`); err != nil {
		t.Fatal(err)
	}
	updated, err := e.correctDocument(e.ForActor("editor").context(context.Background()), doc.ID, old, correctionFor(doc), embed)
	if err != nil {
		t.Fatal(err)
	}
	events, err := s.CollectionAudit("owner", policy.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.Action == "extraction_corrected" {
			count++
			if event.Actor != "editor" || event.Subject != doc.ID+"@"+updated.ExtractionRevision {
				t.Fatal(event)
			}
		}
	}
	if count != 1 {
		t.Fatal("correction audit count", count)
	}
}

func TestExpiryAuditSurvivesRestartWithoutDocument(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := originalFixture([]byte("confidential original"))
	if err = s.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	policy, err := s.CreateCollection("owner", "Records", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", doc.ID, policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetDocumentRetention("owner", doc.ID, 1, time.Now().Add(-time.Hour).Unix(), false); err != nil {
		t.Fatal(err)
	}
	if n, err := s.expireDocuments(time.Now(), 10); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	s.Close()
	s, err = NewSQLiteStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, err := s.CollectionAudit("owner", policy.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.Action != "document_expired" || last.Actor != "system:retention" || last.Subject != doc.ID {
		t.Fatal(last)
	}
}
