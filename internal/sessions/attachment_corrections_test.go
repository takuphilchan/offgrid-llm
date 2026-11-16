package sessions

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func correctionForNote(graph *document.Extraction, text string) []document.Correction {
	for _, block := range graph.Blocks {
		if block.ID == "note" {
			return []document.Correction{{BlockID: block.ID, ExpectedSHA256: document.TextSHA256(graph.Text[block.Span.Start:block.Span.End]), Text: text}}
		}
	}
	return nil
}

func TestPrivateCorrectionRetainsOriginalOldCitationsAndRestartHistory(t *testing.T) {
	scope, directory, old, graph := preparedStructuredAttachment(t)
	next, err := scope.CorrectAttachment(context.Background(), "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "Rural exception: August."))
	if err != nil || next.Revision == old.Revision || next.ParentRevision != old.Revision || next.SHA256 != old.SHA256 || next.CorrectionAuthor != "alice" || next.CorrectedAt == nil {
		t.Fatal(next, err)
	}
	scope = NewSessionManager(directory).WithAccess(Access{UserID: "alice"})
	for _, row := range []struct {
		attachment Attachment
		word       string
	}{{old, "July"}, {next, "August"}} {
		sources, err := scope.AttachmentSources("policy", []AttachmentReference{{ID: row.attachment.ID, Revision: row.attachment.Revision}})
		if err != nil || len(sources) != 1 || !strings.Contains(sources[0].Text, row.word) || sources[0].Structure.Verify() != nil {
			t.Fatal("revision lost after restart", sources, err)
		}
		passage, err := scope.ReadAttachmentPassage("policy", AttachmentReference{ID: old.ID, Revision: row.attachment.Revision}, 0, len(sources[0].Text))
		if err != nil || !strings.Contains(passage.Text, row.word) {
			t.Fatal("citation changed", passage, err)
		}
	}
	_, original, err := scope.AttachmentOriginal("policy", old.ID)
	if err != nil || string(original) != graph.Text {
		t.Fatal("original overwritten", err)
	}
	listed, err := scope.ListAttachments("policy")
	if err != nil || len(listed) != 1 || listed[0].Revision != next.Revision {
		t.Fatal("revision sidecars leaked into library", listed, err)
	}
	if _, err = scope.CorrectAttachment(context.Background(), "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "stale")); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("stale update accepted", err)
	}
	if err = scope.DeleteAttachment("policy", old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = scope.AttachmentSources("policy", []AttachmentReference{{ID: old.ID, Revision: old.Revision}}); err == nil {
		t.Fatal("deleted history resurrected")
	}
}

func TestPrivateCorrectionRequiresOwnerAndExactCurrentEvidence(t *testing.T) {
	scope, directory, old, graph := preparedStructuredAttachment(t)
	for _, access := range []Access{{UserID: "bob"}, {UserID: "admin", All: true}} {
		other := NewSessionManager(directory).WithAccess(access)
		if _, err := other.CorrectAttachment(context.Background(), "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "wrong")); err == nil {
			t.Fatal("nonowner corrected private evidence")
		}
	}
	for _, edits := range [][]document.Correction{nil, {{BlockID: "note", ExpectedSHA256: strings.Repeat("0", 64), Text: "bad"}}, {{BlockID: "clause", ExpectedSHA256: document.TextSHA256("Applications close in June.\n"), Text: "overwrite exception"}}} {
		if _, err := scope.CorrectAttachment(context.Background(), "policy", old.ID, old.Revision, graph.ID, edits); !errors.Is(err, ErrAttachmentCorrectionInvalid) {
			t.Fatal("invalid correction accepted", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scope.CorrectAttachment(ctx, "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "cancelled")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	current, _, err := scope.ReadAttachment("policy", old.ID)
	if err != nil || current.Revision != old.Revision {
		t.Fatal("failed correction mutated current source", err)
	}
}

type cancelBeforePublication struct {
	context.Context
	calls int
}

func (c *cancelBeforePublication) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func TestPrivateCorrectionUnpublishedArtifactsAreUnreachableAndCleaned(t *testing.T) {
	scope, _, old, graph := preparedStructuredAttachment(t)
	_, err := scope.CorrectAttachment(&cancelBeforePublication{Context: context.Background()}, "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "not published"))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	root, dir, err := scope.attachmentRoot("policy", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	folder, err := root.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := folder.ReadDir(-1)
	folder.Close()
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, entry := range entries {
		prefix := old.ID + ".revision-"
		if strings.HasPrefix(entry.Name(), prefix) {
			revision := strings.TrimPrefix(entry.Name(), prefix)
			if revision == old.Revision {
				continue
			}
			found++
			if _, err := readAttachmentRecordRevision(root, dir, old.ID, revision); err == nil {
				t.Fatal("unpublished artifact admitted")
			}
		}
	}
	if found != 1 {
		t.Fatal("expected one durable unpublished revision", found)
	}
	current, _, err := scope.ReadAttachment("policy", old.ID)
	if err != nil || current.Revision != old.Revision {
		t.Fatal("cancelled publication changed source", err)
	}
	if err = scope.DeleteAttachment("policy", old.ID); err != nil {
		t.Fatal(err)
	}
	folder, err = root.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err = folder.ReadDir(-1)
	folder.Close()
	if err != nil || len(entries) != 0 {
		t.Fatal("revision retention cleanup incomplete", entries, err)
	}
}

func TestPrivateCorrectionRejectsHistoryTampering(t *testing.T) {
	scope, _, old, graph := preparedStructuredAttachment(t)
	next, err := scope.CorrectAttachment(context.Background(), "policy", old.ID, old.Revision, graph.ID, correctionForNote(graph, "corrected"))
	if err != nil {
		t.Fatal(err)
	}
	root, dir, err := scope.attachmentRoot("policy", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	current, err := readAttachmentRecord(root, dir, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.PreviousRevisions[0] = strings.Repeat("a", 64)
	if err = writeAttachmentRecord(root, dir, current); err != nil {
		t.Fatal(err)
	}
	if _, err = scope.AttachmentSources("policy", []AttachmentReference{{ID: old.ID, Revision: next.Revision}}); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("forged history admitted", err)
	}
	if err = root.Remove(revisionArtifactPath(dir, old.ID, next.Revision)); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
