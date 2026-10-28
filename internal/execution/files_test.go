package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Session, Binding, string) {
	t.Helper()
	root := t.TempDir()
	b := Binding{Actor: "alice", Workspace: "service-workspace", Host: "local-host", Session: "session-one", Grant: "grant-one", Revision: 1, Job: "job-one"}
	s, err := OpenSession(Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: Permissions{Read: true, Write: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, b, root
}

func put(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestContextAuthorityAndRevocation(t *testing.T) {
	s, b, _ := fixture(t)
	ctx := context.Background()
	for _, change := range []func(*Binding){func(b *Binding) { b.Actor = "bob" }, func(b *Binding) { b.Workspace = "other" }, func(b *Binding) { b.Host = "other" }, func(b *Binding) { b.Grant = "other" }, func(b *Binding) { b.Revision++ }, func(b *Binding) { b.Job = "unadmitted" }} {
		wrong := b
		change(&wrong)
		if _, err := s.Snapshot(ctx, wrong); !errors.Is(err, ErrAuthority) {
			t.Fatalf("authority: %v", err)
		}
	}
	next := b
	next.Job = "job-two"
	if err := s.AdmitJob(next); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(ctx, next); err != nil {
		t.Fatal(err)
	}
	s.Revoke()
	if _, err := s.List(ctx, b, ".", 0, 10, ""); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
}

func TestContextDirectoriesAreSessionLocal(t *testing.T) {
	s, b, root := fixture(t)
	other, ob, oroot := fixture(t)
	if err := os.Mkdir(filepath.Join(root, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, root, "a.txt", "one")
	put(t, oroot, "b.txt", "two")
	before, _ := os.Getwd()
	if _, err := s.ChangeDirectory(context.Background(), b, "child"); err != nil {
		t.Fatal(err)
	}
	a, err := s.List(context.Background(), b, ".", 0, 10, "")
	if err != nil || len(a.Entries) != 0 {
		t.Fatal(a, err)
	}
	z, err := other.List(context.Background(), ob, ".", 0, 10, "")
	if err != nil || len(z.Entries) != 1 || z.Entries[0].Name != "b.txt" {
		t.Fatal(z, err)
	}
	after, _ := os.Getwd()
	if before != after {
		t.Fatal("changed process cwd")
	}
}

func TestListingIsSortedBoundedAndRevisioned(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	put(t, root, "z.txt", "z")
	put(t, root, "a.txt", "a")
	page, err := s.List(ctx, b, ".", 0, 1, "")
	if err != nil || page.Complete || page.NextOffset == nil || page.Entries[0].Name != "a.txt" || page.Host != b.Host || page.Offset != 0 || page.TotalEntries != 2 {
		t.Fatal(page, err)
	}
	page2, err := s.List(ctx, b, ".", *page.NextOffset, 1, page.Revision)
	if err != nil || !page2.Complete || page2.Entries[0].Name != "z.txt" || page2.Offset != 1 || page2.TotalEntries != 2 {
		t.Fatal(page2, err)
	}
	put(t, root, "new.txt", "new")
	if _, err := s.List(ctx, b, ".", 1, 1, page.Revision); !errors.Is(err, ErrRevision) {
		t.Fatal(err)
	}
	if _, err := s.List(ctx, b, ".", 1, 1, ""); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err := s.List(ctx, b, ".", 0, MaxEntriesPerPage+1, ""); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestRawUnicodePagesAndChangedRead(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	content := "1| café 日本語 — 🙂\r\n"
	put(t, root, "source.txt", content)
	p, err := s.Read(ctx, b, "source.txt", 0, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte{}, p.Data...)
	for p.NextOffset != nil {
		p, err = s.Read(ctx, b, "source.txt", *p.NextOffset, 3, p.Revision)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, p.Data...)
	}
	if string(data) != content {
		t.Fatalf("changed bytes: %q", data)
	}
	put(t, root, "source.txt", "changed")
	if _, err = s.Read(ctx, b, "source.txt", 1, 3, p.Revision); !errors.Is(err, ErrRevision) {
		t.Fatal(err)
	}
}

func TestPathsAndUnavailableFiles(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	for _, path := range []string{"../secret", "/etc/passwd", `C:\Users\private`, "file:data", `\\host\share`, string([]byte{255}), "a\x00b"} {
		if _, err := s.Read(ctx, b, path, 0, 10, ""); !errors.Is(err, ErrPath) {
			t.Errorf("path %q: %v", path, err)
		}
	}
	if _, err := s.Read(ctx, b, ".", 0, 10, ""); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	outside := t.TempDir()
	put(t, outside, "secret", "private")
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "link")); err == nil {
		if _, err := s.Read(ctx, b, "link", 0, 10, ""); err == nil {
			t.Fatal("outside symlink read")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.List(canceled, b, ".", 0, 10, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s.config.Permissions.Read = false
	if _, err := s.Read(ctx, b, "missing", 0, 10, ""); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
}

func TestInvalidContexts(t *testing.T) {
	s, b, root := fixture(t)
	_ = s
	for _, config := range []Context{
		{Version: 2, Binding: b, Backend: "local", Root: root},
		{Version: 1, Binding: b, Backend: "local", Root: "relative"},
		{Version: 1, Binding: b, Backend: "docker", Root: root},
		{Version: 1, Binding: b, Backend: "local", Root: root, Directory: "../escape"},
	} {
		if bad, err := OpenSession(config); err == nil {
			bad.Close()
			t.Fatal("invalid context accepted")
		}
	}
	b.Actor = strings.Repeat("x", 257)
	if bad, err := OpenSession(Context{Version: 1, Binding: b, Backend: "local", Root: root}); err == nil {
		bad.Close()
		t.Fatal("invalid identity accepted")
	}
}

func TestReplacedWorkspaceRootCannotChangeExecutionLocation(t *testing.T) {
	s, b, root := fixture(t)
	put(t, root, "private.txt", "original root")
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Skip("platform refused renaming an open fixture directory:", err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	put(t, root, "replacement.txt", "another directory")
	if _, err := s.List(context.Background(), b, ".", 0, 10, ""); !errors.Is(err, ErrPath) {
		t.Fatal("followed replaced root", err)
	}
}
