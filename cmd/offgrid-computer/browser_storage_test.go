package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/testhost"
)

func browserStorageTestDir(t *testing.T) string {
	t.Helper()
	// macOS temporary storage may use the /var -> /private/var system alias.
	// The supervisor supplies canonical owned paths; explicit link tests below
	// separately verify that the checker refuses a newly introduced alias.
	parent, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil || execution.CheckPrivateAncestry(parent) != nil {
		parent, err = os.UserHomeDir()
		if err == nil {
			parent, err = filepath.EvalSymlinks(parent)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := execution.CheckPrivateAncestry(parent); err != nil {
		t.Fatalf("no safe isolated storage fixture parent: %v", err)
	}
	directory, err := os.MkdirTemp(parent, ".offgrid-browser-storage-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(directory) != parent || !strings.HasPrefix(filepath.Base(directory), ".offgrid-browser-storage-test-") {
			t.Fatal("fixture cleanup escaped owner")
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	if err := execution.ProtectPrivatePath(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestBrowserStorageWorkerProcess(t *testing.T) {
	if os.Getenv("OFFGRID_STORAGE_TEST_CHILD") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("storage-worker", flag.ContinueOnError)
	os.Args = []string{os.Args[0], "--browser-storage", "--state", os.Getenv("OFFGRID_STORAGE_TEST_DIRECTORY")}
	main()
	os.Exit(0)
}

func browserStorageFrame(body string) []byte {
	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(frame, uint32(len(body)))
	copy(frame[4:], body)
	return frame
}

func TestBrowserStorageWorkerStartup(t *testing.T) {
	testhost.Account(t)
	directory := filepath.Join(browserStorageTestDir(t), "computer")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrowserStorageWorkerProcess$")
	cmd.Env = append(os.Environ(), "OFFGRID_STORAGE_TEST_CHILD=1", "OFFGRID_STORAGE_TEST_DIRECTORY="+directory)
	cmd.Stdin = bytes.NewReader(browserStorageFrame(`{"protocol":1,"kind":"prepare_browser_storage","workspace_roots":[]}`))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("storage-only worker failed: %v; stderr=%s; output=%q", err, stderr.String(), output)
	}
	var reply struct {
		Protocol int    `json:"protocol"`
		ID       string `json:"id"`
		Result   struct {
			Ready          bool `json:"ready"`
			BrowserStorage int  `json:"browser_storage"`
		} `json:"result"`
	}
	reader := bytes.NewReader(output)
	if err := computer.ReadControlFrame(reader, &reply); err != nil || reader.Len() != 0 || reply.Protocol != 1 || reply.ID != "browser-storage" || !reply.Result.Ready || reply.Result.BrowserStorage != 1 {
		t.Fatalf("unexpected storage handshake: %+v, %v, output=%q", reply, err, output)
	}
	for _, path := range []string{directory, filepath.Join(directory, "uploads")} {
		if err := execution.CheckPrivatePath(path); err != nil {
			t.Fatalf("unprotected storage: %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "dispatch.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("storage check opened a journal: %v", err)
	}
}

func TestBrowserStorageRejectsUnsealedAndMalformedRequests(t *testing.T) {
	valid := `{"protocol":1,"kind":"prepare_browser_storage","workspace_roots":[]}`
	cases := map[string][]byte{
		"missing roots":   browserStorageFrame(`{"protocol":1,"kind":"prepare_browser_storage"}`),
		"null roots":      browserStorageFrame(`{"protocol":1,"kind":"prepare_browser_storage","workspace_roots":null}`),
		"relative root":   browserStorageFrame(`{"protocol":1,"kind":"prepare_browser_storage","workspace_roots":["relative"]}`),
		"wrong protocol":  browserStorageFrame(strings.Replace(valid, `"protocol":1`, `"protocol":2`, 1)),
		"duplicate key":   browserStorageFrame(strings.Replace(valid, `"protocol":1`, `"protocol":1,"protocol":1`, 1)),
		"wrong operation": browserStorageFrame(strings.Replace(valid, "prepare_browser_storage", "execute", 1)),
		"unknown path":    browserStorageFrame(strings.Replace(valid, `"kind":`, `"path":"ignored","kind":`, 1)),
		"trailing JSON":   browserStorageFrame(valid + `{}`),
		"second frame":    append(browserStorageFrame(valid), browserStorageFrame(valid)...),
		"oversize frame":  {0, 0, 64, 1},
		"empty frame":     {0, 0, 0, 0},
		"truncated":       {0, 0, 0, 1},
		"invalid UTF-8":   browserStorageFrame(strings.Replace(valid, "prepare_browser_storage", "prepare_browser_\xffstorage", 1)),
	}
	roots := make([]string, 17)
	for i := range roots {
		roots[i] = browserStorageTestDir(t)
	}
	tooMany, _ := json.Marshal(map[string]any{"protocol": 1, "kind": "prepare_browser_storage", "workspace_roots": roots})
	cases["too many roots"] = browserStorageFrame(string(tooMany))
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(browserStorageTestDir(t), "computer")
			var output bytes.Buffer
			if err := serveBrowserStorage(context.Background(), io.NopCloser(bytes.NewReader(raw)), &output, directory); err == nil {
				t.Fatal("accepted malformed storage request")
			}
			if output.Len() != 0 {
				t.Fatal("published readiness")
			}
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatalf("invalid request mutated storage: %v", err)
			}
		})
	}
	// The complete frame alone is not authority: no work begins until EOF.
	input, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer writer.Close()
	directory := filepath.Join(browserStorageTestDir(t), "computer")
	done := make(chan error, 1)
	go func() { done <- serveBrowserStorage(ctx, input, io.Discard, directory) }()
	if _, err := writer.Write(browserStorageFrame(valid)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("unsealed request created storage")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled request succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled input did not unblock")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("cancellation created storage")
	}
}

func TestBrowserStorageWorkspaceAdmissionPrecedesMutation(t *testing.T) {
	parent := browserStorageTestDir(t)
	directory := filepath.Join(parent, "computer")
	if err := prepareBrowserStorage(context.Background(), directory, []string{parent}); err == nil {
		t.Fatal("admitted workspace ancestor")
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("overlap created private state")
	}
	if err := prepareBrowserStorage(context.Background(), directory, []string{browserStorageTestDir(t)}); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{directory, filepath.Join(directory, "uploads")} {
		if err := prepareBrowserStorage(context.Background(), directory, []string{root}); err == nil {
			t.Fatalf("admitted overlapping root %s", root)
		}
	}
	if err := prepareBrowserStorage(context.Background(), directory, []string{filepath.Join(browserStorageTestDir(t), "missing")}); err == nil {
		t.Fatal("accepted unresolvable approved root")
	}
}

func browserStorageFixture(t *testing.T) (string, string, string) {
	t.Helper()
	directory := filepath.Join(browserStorageTestDir(t), "computer")
	if err := prepareBrowserStorage(context.Background(), directory, nil); err != nil {
		t.Fatal(err)
	}
	resource := filepath.Join(directory, "uploads", strings.Repeat("a", 32))
	if err := execution.EnsurePrivateDirectory(resource); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(resource, "fixture.txt")
	if err := os.WriteFile(file, []byte("retained approved bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := execution.ProtectPrivatePath(file); err != nil {
		t.Fatal(err)
	}
	return directory, resource, file
}

func TestBrowserStoragePreservesRetainedContentsWithoutRepair(t *testing.T) {
	for _, kind := range []string{"root", "uploads", "resource", "snapshot", "journal", "sidecar"} {
		t.Run(kind, func(t *testing.T) {
			directory, resource, file := browserStorageFixture(t)
			if err := prepareBrowserStorage(context.Background(), directory, nil); err != nil {
				t.Fatal(err)
			}
			path := map[string]string{"root": directory, "uploads": filepath.Join(directory, "uploads"), "resource": resource, "snapshot": file, "journal": filepath.Join(directory, "dispatch.sqlite"), "sidecar": filepath.Join(directory, "dispatch.sqlite-journal")}[kind]
			if kind == "journal" || kind == "sidecar" {
				if err := os.WriteFile(path, []byte("preserve journal"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			makeBrowserStoragePublic(t, path)
			if err := prepareBrowserStorage(context.Background(), directory, nil); err == nil {
				t.Fatal("accepted public existing storage")
			}
			if err := execution.CheckPrivatePath(path); err == nil {
				t.Fatal("silently repaired existing permissions")
			}
			if data, err := os.ReadFile(file); err != nil || string(data) != "retained approved bytes" {
				t.Fatalf("retained file changed: %q %v", data, err)
			}
			if kind == "journal" || kind == "sidecar" {
				if data, err := os.ReadFile(path); err != nil || string(data) != "preserve journal" {
					t.Fatal("journal changed")
				}
			}
		})
	}
}

func TestBrowserStorageRejectsUnexpectedDescendants(t *testing.T) {
	for _, kind := range []string{"unknown resource", "second file", "nested directory", "directory journal", "too many resources"} {
		t.Run(kind, func(t *testing.T) {
			directory, resource, _ := browserStorageFixture(t)
			switch kind {
			case "unknown resource":
				if err := os.Mkdir(filepath.Join(directory, "uploads", "unowned"), 0700); err != nil {
					t.Fatal(err)
				}
			case "second file":
				if err := os.WriteFile(filepath.Join(resource, "extra"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "nested directory":
				if err := os.Remove(filepath.Join(resource, "fixture.txt")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(resource, "nested"), 0700); err != nil {
					t.Fatal(err)
				}
			case "directory journal":
				if err := os.Mkdir(filepath.Join(directory, "dispatch.sqlite"), 0700); err != nil {
					t.Fatal(err)
				}
			case "too many resources":
				// The enumeration limit is independent of valid names/content.
				for i := 0; i < 1024; i++ {
					if err := os.Mkdir(filepath.Join(directory, "uploads", fmt.Sprintf("%032x", i)), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := prepareBrowserStorage(context.Background(), directory, nil); err == nil {
				t.Fatal("accepted unexpected retained layout")
			}
		})
	}
}

func TestBrowserStorageRejectsLinkedStorage(t *testing.T) {
	for _, kind := range []string{"prefix", "uploads", "snapshot", "journal"} {
		t.Run(kind, func(t *testing.T) {
			directory, resource, file := browserStorageFixture(t)
			source, target := directory, filepath.Join(browserStorageTestDir(t), "alias")
			switch kind {
			case "uploads":
				source = filepath.Join(browserStorageTestDir(t), "external")
				if err := os.Mkdir(source, 0700); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(directory, "uploads")
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(resource); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
			case "snapshot":
				source = filepath.Join(browserStorageTestDir(t), "outside.txt")
				if err := os.WriteFile(source, []byte("outside"), 0600); err != nil {
					t.Fatal(err)
				}
				target = file
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			case "journal":
				source = file
				target = filepath.Join(directory, "dispatch.sqlite")
			}
			if err := os.Symlink(source, target); err != nil {
				t.Skipf("isolated symlink unavailable: %v", err)
			}
			if kind == "prefix" {
				directory = filepath.Join(target, "new")
			}
			if err := prepareBrowserStorage(context.Background(), directory, nil); err == nil {
				t.Fatal("accepted linked storage")
			}
		})
	}
}
