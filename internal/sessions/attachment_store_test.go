package sessions

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConversationAttachmentsArePrivateAndRevisionBound(t *testing.T) {
	dir := t.TempDir()
	manager := NewSessionManager(dir)
	alice := manager.WithAccess(Access{UserID: "alice"})
	bob := manager.WithAccess(Access{UserID: "bob"})
	if _, err := alice.Create("private", "model"); err != nil {
		t.Fatal(err)
	}
	content := []byte("Private café 日本語 source")
	a, err := alice.CreateAttachment("private", "notes.txt", "text/plain", content)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != "uploaded" || a.Revision != "" {
		t.Fatalf("premature readiness: %+v", a)
	}
	if _, _, err := alice.ReadAttachment("private", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatalf("unprepared file entered context: %v", err)
	}
	if _, _, err := bob.AttachmentOriginal("private", a.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("cross-user original: %v", err)
	}
	if _, err := bob.ListAttachments("private"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("cross-user list: %v", err)
	}
	ready, err := alice.PrepareAttachment("private", a.ID, string(content), "text-v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "ready" || len(ready.Revision) != 64 {
		t.Fatalf("missing extraction revision: %+v", ready)
	}
	manager = NewSessionManager(dir)
	alice = manager.WithAccess(Access{UserID: "alice"})
	_, raw, err := alice.AttachmentOriginal("private", a.ID)
	if err != nil || !bytes.Equal(raw, content) {
		t.Fatalf("original after restart: %q %v", raw, err)
	}
	_, text, err := alice.ReadAttachment("private", a.ID)
	if err != nil || text != string(content) {
		t.Fatalf("source after restart: %q %v", text, err)
	}
	items, err := alice.ListAttachments("private")
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %v", items, err)
	}
	b, _ := json.Marshal(items)
	if bytes.Contains(b, content) {
		t.Fatal("private text leaked into metadata list")
	}
	sessions, _ := alice.List()
	b, _ = json.Marshal(sessions)
	if bytes.Contains(b, content) {
		t.Fatal("private file leaked into session list")
	}
	if _, err := alice.PrepareAttachment("private", a.ID, "changed", "text-v2", ""); !errors.Is(err, ErrTurnConflict) {
		t.Fatalf("ready extraction overwritten: %v", err)
	}
}

func TestAttachmentFailuresAndLimitsDoNotBecomeReady(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	scope := manager.WithAccess(Access{All: true})
	_, _ = scope.Create("test", "model")
	for _, name := range []string{"../secret", "folder/name", "C:\\secret", "bad\x00.txt", strings.Repeat("a", 256)} {
		if _, err := scope.CreateAttachment("test", name, "text/plain", []byte("safe")); !errors.Is(err, ErrAttachmentLimit) {
			t.Fatalf("unsafe filename %q: %v", name, err)
		}
	}
	a, err := scope.CreateAttachment("test", "scan.pdf", "application/pdf", []byte("fixture"))
	if err != nil {
		t.Fatal(err)
	}
	failed, err := scope.PrepareAttachment("test", a.ID, "", "pdf-v1", "document_ocr_required")
	if err != nil || failed.Status != "failed" {
		t.Fatalf("missing failure: %+v %v", failed, err)
	}
	if _, _, err := scope.ReadAttachment("test", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("failed file entered context")
	}
	for i := 0; i < 7; i++ {
		if _, err := scope.CreateAttachment("test", "note.txt", "text/plain", []byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := scope.CreateAttachment("test", "extra.txt", "text/plain", []byte("fixture")); !errors.Is(err, ErrAttachmentLimit) {
		t.Fatalf("count limit: %v", err)
	}
}

func TestAttachmentSourceCorruptionAndRecreatedSession(t *testing.T) {
	dir := t.TempDir()
	manager := NewSessionManager(dir)
	scope := manager.WithAccess(Access{UserID: "alice"})
	_, _ = scope.Create("same", "model")
	a, err := scope.CreateAttachment("same", "note.txt", "text/plain", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = scope.PrepareAttachment("same", a.ID, "original", "text-v1", "")
	var blob string
	_ = filepath.WalkDir(dir, func(p string, entry os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, a.ID+".bin") {
			blob = p
		}
		return err
	})
	if blob == "" {
		t.Fatal("missing original")
	}
	if err := os.WriteFile(blob, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := scope.AttachmentOriginal("same", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatal("changed original accepted")
	}
	if err := scope.Delete("same"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blob); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted conversation retained original bytes: %v", err)
	}
	_, _ = scope.Create("same", "model")
	if _, _, err := scope.ReadAttachment("same", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
		t.Fatalf("recreated session inherited private source: %v", err)
	}
}
