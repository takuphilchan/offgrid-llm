package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExclusiveCreateAndPreconditionedReplace(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	content := []byte("1| café 日本語 🙂\r\n")
	result, err := s.Write(ctx, b, "copy.txt", content, "")
	if err != nil || !result.Created || !result.Verified || result.Revision != digest(content) {
		t.Fatal(result, err)
	}
	if _, err = s.Write(ctx, b, "copy.txt", []byte("overwrite"), ""); !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	put(t, root, "unrelated.txt", "preserve")
	if _, err = s.Write(ctx, b, "copy.txt", []byte("new"), strings.Repeat("0", 64)); !errors.Is(err, ErrRevision) {
		t.Fatal(err)
	}
	result, err = s.Write(ctx, b, "copy.txt", []byte("new"), result.Revision)
	if err != nil || result.Created || !result.Verified {
		t.Fatal(result, err)
	}
	other, err := os.ReadFile(filepath.Join(root, "unrelated.txt"))
	if err != nil || string(other) != "preserve" {
		t.Fatal(string(other), err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatal(entries, err)
	}
}

func TestWriteBoundsAuthorityAndLinks(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	if _, err := s.Write(ctx, b, "../escape", nil, ""); !errors.Is(err, ErrPath) {
		t.Fatal(err)
	}
	if _, err := s.Write(ctx, b, "large", make([]byte, MaxFileBytes+1), ""); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	put(t, root, "source", "before")
	if err := os.Symlink("source", filepath.Join(root, "link")); err == nil {
		if _, err := s.Write(ctx, b, "link", []byte("bad"), digest([]byte("before"))); !errors.Is(err, ErrUnsupported) {
			t.Fatal(err)
		}
	}
	s.config.Permissions.Write = false
	if _, err := s.Write(ctx, b, "new", nil, ""); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
}

func TestLiteralSearchAndTruncation(t *testing.T) {
	s, b, root := fixture(t)
	ctx := context.Background()
	put(t, root, "notes.txt", "one\nneedle 日本語\nneedle two\n")
	result, err := s.Search(ctx, b, ".", "needle", 10)
	if err != nil || !result.Complete || len(result.Matches) != 2 || result.Matches[0].Line != 2 || result.Matches[0].Text != "needle 日本語" {
		t.Fatal(result, err)
	}
	result, err = s.Search(ctx, b, ".", "needle", 1)
	if err != nil || result.Complete || len(result.Matches) != 1 {
		t.Fatal(result, err)
	}
	put(t, root, "long.txt", "needle"+strings.Repeat("a", 1200))
	long, err := s.Search(ctx, b, "long.txt", "needle", 10)
	if err != nil || len(long.Matches) != 1 || !long.Matches[0].Truncated {
		t.Fatal(long, err)
	}
	put(t, root, "binary", string([]byte{0, 1, 2}))
	result, err = s.Search(ctx, b, ".", "absent", 10)
	if err != nil || result.Complete || result.Skipped != 1 {
		t.Fatal(result, err)
	}
}
