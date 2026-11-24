package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedHostStateRejectsOverlappingWorkspace(t *testing.T) {
	parent := t.TempDir()
	private := filepath.Join(parent, "computer")
	child := filepath.Join(private, "uploads")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{parent, private, child} {
		if err := PrivateStateDisjointFromRoot(root, private); !errors.Is(err, ErrAuthority) {
			t.Fatalf("overlap %s: %v", root, err)
		}
	}
	disjoint := t.TempDir()
	if err := PrivateStateDisjointFromRoot(disjoint, private, filepath.Join(parent, "new", "state")); err != nil {
		t.Fatal(err)
	}
	if err := PrivateStateDisjointFromRoot(parent, filepath.Join(parent, "not-created", "state")); !errors.Is(err, ErrAuthority) {
		t.Fatalf("missing state: %v", err)
	}
	if err := PrivateStateDisjointFromRoot(disjoint, "relative"); err == nil {
		t.Fatal("relative protection accepted")
	}
}

func TestProtectedHostStateResolvesAliasesAndRejectsDanglingLinks(t *testing.T) {
	parent := t.TempDir()
	private := filepath.Join(parent, "computer")
	if err := os.MkdirAll(filepath.Join(private, "uploads"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(private, alias); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	for _, pair := range [][2]string{{alias, private}, {filepath.Join(alias, "uploads"), private}, {parent, filepath.Join(alias, "future")}} {
		if err := PrivateStateDisjointFromRoot(pair[0], pair[1]); !errors.Is(err, ErrAuthority) {
			t.Fatalf("alias overlap: %v", err)
		}
	}
	dangling := filepath.Join(parent, "dangling")
	if err := os.Symlink(filepath.Join(parent, "absent"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := PrivateStateDisjointFromRoot(t.TempDir(), filepath.Join(dangling, "future")); err == nil {
		t.Fatal("dangling alias accepted")
	}
}

func TestNativeHostReservesBrowserStorageBeforePackAndPairing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	custom := t.TempDir()
	t.Setenv("OFFGRID_DESKTOP_HOME", custom)
	extra := filepath.Join(t.TempDir(), "private-browser")
	for _, private := range []string{filepath.Join(home, ".offgrid-llm", "computer"), filepath.Join(custom, "computer"), extra} {
		child := filepath.Join(private, "uploads")
		if err := os.MkdirAll(child, 0700); err != nil {
			t.Fatal(err)
		}
		for _, folder := range []string{filepath.Dir(private), private, child} {
			approved := false
			_, err := StartLocalHost(context.Background(), nil, LocalHostOptions{Folder: folder, PackDirectory: t.TempDir(), StateDirectory: t.TempDir(), ProtectedDirectories: []string{extra}}, func(LocalApproval) bool { approved = true; return true })
			if !errors.Is(err, ErrAuthority) || approved {
				t.Fatalf("unsafe grant %s: err=%v approved=%v", folder, err, approved)
			}
		}
	}
}

func TestProtectedHostDirectoryInputsAreBounded(t *testing.T) {
	for _, paths := range [][]string{{"relative"}, {"\x00"}, make([]string, 17)} {
		if _, err := protectedHostDirectories(paths); err == nil {
			t.Fatal("invalid roots accepted")
		}
	}
}
