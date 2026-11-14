package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func replacementFixture(t *testing.T) (*os.Root, *os.Root, string, string) {
	t.Helper()
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("backend intentionally unsupported")
	}
	directory := t.TempDir()
	targetPath, privatePath := filepath.Join(directory, "target"), filepath.Join(directory, "private")
	for _, name := range []string{targetPath, privatePath} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeCandidateFile(t, targetPath, "nested/input.txt", "original")
	writeCandidateFile(t, privatePath, "staged", "proposed")
	target, err := os.OpenRoot(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	private, err := os.OpenRoot(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { private.Close() })
	return target, private, targetPath, privatePath
}

func TestPreservingReplacementRetainsActualDisplacedFile(t *testing.T) {
	for _, mode := range []string{"normal", "late-edit", "post-edit", "lost-ack", "cancel-after"} {
		t.Run(mode, func(t *testing.T) {
			target, private, targetPath, privatePath := replacementFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dispatch := func(to *replacementDirectory, name string, from *replacementDirectory, staged, backup string) (string, error) {
				if mode == "late-edit" {
					writeCandidateFile(t, targetPath, "nested/input.txt", "user edit after precheck")
				}
				saved, err := replacePinned(to, name, from, staged, backup)
				if err != nil {
					return saved, err
				}
				if mode == "post-edit" {
					writeCandidateFile(t, targetPath, "nested/input.txt", "newer user edit")
				}
				if mode == "lost-ack" {
					return saved, errors.New("injected lost acknowledgment")
				}
				if mode == "cancel-after" {
					cancel()
				}
				return saved, nil
			}
			receipt, err := replacePreservingUsing(ctx, target, "nested/input.txt", private, "staged", "displaced", digest([]byte("original")), digest([]byte("proposed")), 1024, dispatch)
			if !receipt.Dispatched {
				t.Fatal("replacement never dispatched", err)
			}
			if mode == "normal" || mode == "cancel-after" {
				if err != nil || receipt.DisplacedDigest != digest([]byte("original")) || receipt.ResultDigest != digest([]byte("proposed")) {
					t.Fatal(receipt, err)
				}
			} else {
				var mutation *MutationError
				if !errors.As(err, &mutation) {
					t.Fatal("effect was not retained", receipt, err)
				}
				if mode != "lost-ack" && !errors.Is(err, ErrConflict) {
					t.Fatal("external edit not reported", receipt, err)
				}
			}
			wantOld := "original"
			if mode == "late-edit" {
				wantOld = "user edit after precheck"
			}
			old, readErr := os.ReadFile(filepath.Join(privatePath, receipt.Displaced))
			if readErr != nil || string(old) != wantOld {
				t.Fatal("displaced bytes lost", string(old), readErr)
			}
			wantNew := "proposed"
			if mode == "post-edit" {
				wantNew = "newer user edit"
			}
			current, readErr := os.ReadFile(filepath.Join(targetPath, "nested/input.txt"))
			if readErr != nil || string(current) != wantNew {
				t.Fatal("blind rollback changed newer bytes", string(current), readErr)
			}
		})
	}
}

func TestPreservingReplacementRejectsBeforeDispatch(t *testing.T) {
	for _, mode := range []string{"preimage", "stage", "backup-exists", "cancelled", "path"} {
		t.Run(mode, func(t *testing.T) {
			target, private, targetPath, privatePath := replacementFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			name := "nested/input.txt"
			switch mode {
			case "preimage":
				writeCandidateFile(t, targetPath, name, "user edit")
			case "stage":
				writeCandidateFile(t, privatePath, "staged", "different stage")
			case "backup-exists":
				writeCandidateFile(t, privatePath, "displaced", "earlier retained bytes")
			case "cancelled":
				cancel()
			case "path":
				name = "../outside"
			}
			receipt, err := replacePreserving(ctx, target, name, private, "staged", "displaced", digest([]byte("original")), digest([]byte("proposed")), 1024)
			if err == nil || receipt.Dispatched {
				t.Fatal("invalid replacement dispatched", receipt, err)
			}
			want := "original"
			if mode == "preimage" {
				want = "user edit"
			}
			data, err := os.ReadFile(filepath.Join(targetPath, "nested/input.txt"))
			if err != nil || string(data) != want {
				t.Fatal("original changed", string(data), err)
			}
		})
	}
}

func TestPreservingReplacementRejectsLinkedParent(t *testing.T) {
	target, private, targetPath, _ := replacementFixture(t)
	outside := t.TempDir()
	writeCandidateFile(t, outside, "input.txt", "original")
	if err := os.Symlink(outside, filepath.Join(targetPath, "linked")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	receipt, err := replacePreserving(context.Background(), target, "linked/input.txt", private, "staged", "displaced", digest([]byte("original")), digest([]byte("proposed")), 1024)
	if err == nil || receipt.Dispatched {
		t.Fatal("linked parent dispatched", receipt, err)
	}
	data, err := os.ReadFile(filepath.Join(outside, "input.txt"))
	if err != nil || string(data) != "original" {
		t.Fatal("outside file changed", string(data), err)
	}
}
