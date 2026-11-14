//go:build linux

package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPreservingReplacementLateLinkRetainsLinkWithoutFollowing(t *testing.T) {
	target, private, targetPath, privatePath := replacementFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	dispatch := func(to *replacementDirectory, name string, from *replacementDirectory, staged, backup string) (string, error) {
		if err := os.Rename(filepath.Join(targetPath, "nested/input.txt"), filepath.Join(targetPath, "nested/user-saved.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(targetPath, "nested/input.txt")); err != nil {
			t.Fatal(err)
		}
		return replacePinned(to, name, from, staged, backup)
	}
	receipt, err := replacePreservingUsing(context.Background(), target, "nested/input.txt", private, "staged", "displaced", digest([]byte("original")), digest([]byte("proposed")), 1024, dispatch)
	var mutation *MutationError
	if !receipt.Dispatched || !errors.As(err, &mutation) {
		t.Fatal("link substitution was not retained as uncertain", receipt, err)
	}
	link, err := os.Readlink(filepath.Join(privatePath, receipt.Displaced))
	if err != nil || link != outside {
		t.Fatal("displaced link lost", link, err)
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "outside unchanged" {
		t.Fatal("followed outside link", string(data), err)
	}
}

func TestPreservingReplacementPinsDirectoryAcrossRename(t *testing.T) {
	target, private, targetPath, _ := replacementFixture(t)
	dispatch := func(to *replacementDirectory, name string, from *replacementDirectory, staged, backup string) (string, error) {
		if err := os.Rename(filepath.Join(targetPath, "nested"), filepath.Join(targetPath, "moved")); err != nil {
			t.Fatal(err)
		}
		writeCandidateFile(t, targetPath, "nested/input.txt", "new directory contents")
		return replacePinned(to, name, from, staged, backup)
	}
	receipt, err := replacePreservingUsing(context.Background(), target, "nested/input.txt", private, "staged", "displaced", digest([]byte("original")), digest([]byte("proposed")), 1024, dispatch)
	if !errors.Is(err, ErrConflict) || !receipt.Dispatched {
		t.Fatal(receipt, err)
	}
	for name, want := range map[string]string{"nested/input.txt": "new directory contents", "moved/input.txt": "proposed"} {
		data, err := os.ReadFile(filepath.Join(targetPath, name))
		if err != nil || string(data) != want {
			t.Fatal("rebound path was followed", name, string(data), err)
		}
	}
}
