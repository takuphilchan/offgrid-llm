package sessions

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

func TestAttachmentOCRCheckpointRestartIsolationAndDeletion(t *testing.T) {
	directory := t.TempDir()
	s := NewSessionManager(directory).WithAccess(Access{UserID: "alice"})
	if _, err := s.Create("scan", "model"); err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateAttachment("scan", "scan.png", "image/png", []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	store := s.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256)
	if raw, err := store.Read(1, key); err != nil || raw != nil {
		t.Fatal(raw, err)
	}
	if err := store.Write(1, key, []byte("{\n\"text\": \"<retained>\"}\n")); err != nil {
		t.Fatal(err)
	}
	s = NewSessionManager(directory).WithAccess(Access{UserID: "alice"})
	store = s.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256)
	if raw, err := store.Read(1, key); err != nil || !bytes.Equal(raw, []byte(`{"text":"<retained>"}`)) {
		t.Fatal(string(raw), err)
	}
	if raw, err := store.Read(1, strings.Repeat("b", 64)); err != nil || raw != nil {
		t.Fatal("wrong raster reused", err)
	}
	other := NewSessionManager(directory).WithAccess(Access{UserID: "bob"})
	if _, err := other.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256).Read(1, key); err == nil {
		t.Fatal("owner boundary bypassed")
	}
	if _, err := s.AttachmentOCRCheckpoints("scan", a.ID, strings.Repeat("0", 64)).Read(1, key); err == nil {
		t.Fatal("source boundary bypassed")
	}
	if err := store.Write(0, key, []byte(`{}`)); !errors.Is(err, documentocr.ErrCheckpointInvalid) {
		t.Fatal(err)
	}
	if err := s.DeleteAttachment("scan", a.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(1, key, []byte(`{}`)); err == nil {
		t.Fatal("stale worker resurrected removed source")
	}
	root, dir, err := s.attachmentRoot("scan", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.Lstat(dir + "/" + a.ID + ".ocr-0001"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("checkpoint not removed", err)
	}
}

func TestAttachmentOCRCheckpointCorruptionFailsClosed(t *testing.T) {
	s := NewSessionManager(t.TempDir()).WithAccess(Access{UserID: "alice"})
	_, _ = s.Create("scan", "model")
	a, err := s.CreateAttachment("scan", "scan.png", "image/png", []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	store, key := s.AttachmentOCRCheckpoints("scan", a.ID, a.SHA256), strings.Repeat("a", 64)
	if err := store.Write(1, key, []byte(`{"text":"retained"}`)); err != nil {
		t.Fatal(err)
	}
	root, dir, err := s.attachmentRoot("scan", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path := dir + "/" + a.ID + ".ocr-0001"
	data, err := root.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("retained"), []byte("tampered"))
	if err := root.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(1, key); !errors.Is(err, documentocr.ErrCheckpointInvalid) {
		t.Fatal("corrupt evidence used", err)
	}
}
