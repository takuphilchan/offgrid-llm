package verification

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestContractsRejectUnknownVersionsAndAmbiguousPaths(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testFile(t, root, "source.txt", []byte("original"))
	valid := Contract{Version: Version, Kind: ExactCopy, Workspace: root, Source: "source.txt", Destination: "copy.txt"}
	cases := map[string]func(*Contract){
		"version":               func(c *Contract) { c.Version++ },
		"kind":                  func(c *Contract) { c.Kind = "copy-using-model-prose" },
		"relative-workspace":    func(c *Contract) { c.Workspace = "." },
		"same-path":             func(c *Contract) { c.Destination = c.Source },
		"case-alias":            func(c *Contract) { c.Destination = "SOURCE.TXT" },
		"absent-source":         func(c *Contract) { c.Source = "missing" },
		"absent-parent":         func(c *Contract) { c.Destination = "missing/copy.txt" },
		"directory-source":      func(c *Contract) { c.Source = "." },
		"no-changes-copy-paths": func(c *Contract) { c.Kind = NoChanges },
	}
	for _, path := range []string{"../escape", "/escape", `..\escape`, "safe/../escape", "C:/escape", "a//b", "file:stream", "a\x00b"} {
		cases[path] = func(c *Contract) { c.Destination = path }
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid
			mutate(&c)
			if v, err := New(background, c); err == nil {
				v.Close()
				t.Fatal("accepted invalid contract")
			}
		})
	}
}

func TestWorkspaceRejectsLinksAndReplacement(t *testing.T) {
	for _, kind := range []string{"symlink-file", "symlink-directory", "hardlink", "new-symlink", "root-replaced"} {
		t.Run(kind, func(t *testing.T) {
			v, root := testVerifier(t, NoChanges, []byte("original"))
			switch kind {
			case "hardlink":
				if err := os.Link(filepath.Join(root, "source.txt"), filepath.Join(root, "linked")); err != nil {
					t.Fatal(err)
				}
			case "root-replaced":
				moved := root + "-moved"
				if err := os.Rename(root, moved); err != nil {
					// Windows Root retains a directory handle without delete
					// sharing; replacement is prevented by the OS itself.
					if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
						if r := v.Finish(background, "completed"); r.State != "passed" {
							t.Fatal(r)
						}
						return
					}
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				// Restore only this test-owned directory so TempDir cleanup owns it.
				t.Cleanup(func() { v.Close(); os.Remove(root); os.Rename(moved, root) })
			default:
				outside := t.TempDir()
				target := filepath.Join(outside, "private")
				testFile(t, outside, "private", []byte("outside-private-data"))
				if kind == "symlink-directory" {
					target = outside
				}
				if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
					t.Skipf("native symlink creation unavailable: %v", err)
				}
			}
			if r := v.Finish(background, "completed"); r.State != "failed" {
				t.Fatal(r)
			}
			if kind != "root-replaced" && kind != "new-symlink" {
				if next, err := New(background, Contract{Version: Version, Kind: NoChanges, Workspace: root}); err == nil {
					next.Close()
					t.Fatal("accepted linked workspace")
				}
			}
		})
	}
}

func TestWorkspaceBoundsAndCancellation(t *testing.T) {
	for _, kind := range []string{"entries", "file-size", "total-bytes", "depth", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			ctx := background
			switch kind {
			case "entries":
				for i := range maxEntries {
					testFile(t, root, fmt.Sprint(i), nil)
				}
			case "file-size":
				testFile(t, root, "oversize", bytes.Repeat([]byte("x"), maxFileBytes+1))
			case "total-bytes":
				for i := range 9 {
					testFile(t, root, fmt.Sprint(i), bytes.Repeat([]byte("x"), maxFileBytes))
				}
			case "depth":
				if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(strings.Repeat("nested/", 18))), 0700); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				cancelled, cancel := context.WithCancel(background)
				cancel()
				ctx = cancelled
			}
			if v, err := New(ctx, Contract{Version: Version, Kind: NoChanges, Workspace: root}); err == nil {
				v.Close()
				t.Fatal("accepted unbounded/cancelled snapshot")
			}
		})
	}
}

func TestExistingDestinationPreservedUntilExternalWrite(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testFile(t, root, "source.txt", []byte("exact"))
	testFile(t, root, "destination.txt", []byte("existing"))
	v, err := New(background, Contract{Version: Version, Kind: ExactCopy, Workspace: root, Source: "source.txt", Destination: "destination.txt"})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Bind(v.RunID(), "root-session")
	read(t, v, "read-source", "source.txt", true)
	if data, err := os.ReadFile(filepath.Join(root, "destination.txt")); err != nil || string(data) != "existing" {
		t.Fatal("verifier changed existing destination")
	}
	write(t, v, root, []byte("exact"))
	read(t, v, "read-back", "destination.txt", true)
	if r := v.Finish(background, "completed"); r.State != "passed" {
		t.Fatal(r)
	}
}

func TestSourceChangeAndOutOfScopeOperationsAreNotApproved(t *testing.T) {
	for _, path := range []string{"../outside.txt", "/absolute.txt", "source.txt", "other.txt"} {
		v, _ := testVerifier(t, ExactCopy, []byte("exact"))
		read(t, v, "read-source", "source.txt", true)
		if d := v.Before(background, event(v, "write"), Operation{Kind: "write", Path: path, Content: []byte("exact")}); d.Allowed {
			t.Fatal(d)
		}
	}
	v, root := testVerifier(t, ExactCopy, []byte("exact"))
	read(t, v, "read-source", "source.txt", true)
	testFile(t, root, "source.txt", []byte("changed"))
	if d := v.Before(background, event(v, "write"), Operation{Kind: "write", Path: "destination.txt", Content: []byte("exact")}); d.Allowed {
		t.Fatal(d)
	}
}

func TestCallsAreBoundedAndInterleavingIsRejected(t *testing.T) {
	v, _ := testVerifier(t, NoChanges, []byte("exact"))
	for i := range maxCalls {
		read(t, v, fmt.Sprint(i), "source.txt", true)
	}
	if d := v.Before(background, event(v, "overflow"), Operation{Kind: "read", Path: "source.txt"}); d.Allowed || d.Code != "event_limit_exceeded" {
		t.Fatal(d)
	}
	if r := v.Finish(background, "completed"); r.State != "failed" {
		t.Fatal(r)
	}
	other, _ := testVerifier(t, NoChanges, []byte("exact"))
	if d := other.Before(background, event(other, "first"), Operation{Kind: "read", Path: "source.txt"}); !d.Allowed {
		t.Fatal(d)
	}
	if d := other.Before(background, event(other, "second"), Operation{Kind: "read", Path: "source.txt"}); d.Allowed || d.Code != "operation_in_flight" {
		t.Fatal(d)
	}
}
