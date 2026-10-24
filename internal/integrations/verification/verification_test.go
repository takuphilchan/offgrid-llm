package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var background = context.Background()

func testFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func testVerifier(t *testing.T, kind string, data []byte) (*Verifier, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testFile(t, root, "source.txt", data)
	testFile(t, root, "untouched.txt", []byte("preserve"))
	c := Contract{Version: Version, Kind: kind, Workspace: root}
	if kind == ExactCopy {
		c.Source, c.Destination = "source.txt", "destination.txt"
	}
	v, err := New(background, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	if err := v.Bind(v.RunID(), "root-session"); err != nil {
		t.Fatal(err)
	}
	return v, root
}

func event(v *Verifier, id string) Identity {
	return Identity{Run: v.RunID(), Session: "root-session", Turn: "turn-1", Call: id}
}

func read(t *testing.T, v *Verifier, id, path string, complete bool) {
	t.Helper()
	if d := v.Before(background, event(v, id), Operation{Kind: "read", Path: path}); !d.Allowed {
		t.Fatal(d)
	}
	if d := v.After(background, event(v, id), Completion{Status: "completed", CompleteRead: complete}); !d.Allowed {
		t.Fatal(d)
	}
}

func write(t *testing.T, v *Verifier, root string, data []byte) {
	t.Helper()
	if d := v.Before(background, event(v, "write"), Operation{Kind: "write", Path: "destination.txt", Content: data}); !d.Allowed {
		t.Fatal(d)
	}
	// Only the test's simulated external tool writes. The verifier never does.
	testFile(t, root, "destination.txt", data)
	if d := v.After(background, event(v, "write"), Completion{Status: "completed"}); !d.Allowed {
		t.Fatal(d)
	}
}

func TestProvenDisplayPrefixRecoveryPreservesGenuineContent(t *testing.T) {
	for _, content := range []string{"OffGrid — café 日本語", "1|genuine prefix\r\n", "", "123|actual data"} {
		v, root := testVerifier(t, ExactCopy, []byte(content))
		read(t, v, "source", "source.txt", true)
		proposal := []byte("1|" + content)
		d := v.Before(background, event(v, "bad-write"), Operation{Kind: "write", Path: "destination.txt", Content: proposal})
		if d.Allowed || d.Code != "content_mismatch" || !strings.Contains(d.Message, "2 extra leading display bytes") {
			t.Fatal(d)
		}
		if string(proposal) != "1|"+content {
			t.Fatal("proposal changed")
		}
		v.After(background, event(v, "bad-write"), Completion{Status: "blocked"})
		write(t, v, root, []byte(content))
		read(t, v, "read-back", "destination.txt", true)
		if r := v.Finish(background, "completed"); r.State != "passed" {
			t.Fatal(r)
		}
	}
	v, _ := testVerifier(t, ExactCopy, []byte("1|genuine"))
	read(t, v, "source", "source.txt", true)
	d := v.Before(background, event(v, "bad"), Operation{Kind: "write", Path: "destination.txt", Content: []byte("genuine")})
	if d.Allowed || d.Message != "" {
		t.Fatal("guessed repair for removed genuine prefix", d)
	}
}

func TestExactBytesAndIndependentReadback(t *testing.T) {
	for _, content := range []string{"", "OffGrid test — café 日本語 12345.", "1|genuine numbered source\r\n2|second line\n", "  whitespace\t\n\n"} {
		t.Run(fmt.Sprintf("bytes-%d", len(content)), func(t *testing.T) {
			v, root := testVerifier(t, ExactCopy, []byte(content))
			read(t, v, "read-source", "source.txt", true)
			write(t, v, root, []byte(content))
			read(t, v, "read-back", "destination.txt", true)
			r := v.Finish(background, "completed")
			if r.State != "passed" || r.CheckedEntries != 4 || r.ObservedCalls != 3 {
				t.Fatal(r)
			}
			if again := v.Finish(background, "failed"); again != r {
				t.Fatal("receipt was not frozen")
			}
			if d := v.Before(background, event(v, "late"), Operation{Kind: "write", Path: "destination.txt"}); d.Allowed {
				t.Fatal("late event allowed")
			}
		})
	}
}

func TestWrongContentIsRejectedWithoutRewriting(t *testing.T) {
	want := []byte("1|café 日本語\n")
	for _, wrong := range []string{"1|1|café 日本語\n", `{"name":"read_file","arguments":{"path":"source.txt"}}`, "café 日本語\n", "1|cafe\u0301 日本語\n", "1|café"} {
		t.Run(wrong, func(t *testing.T) {
			v, root := testVerifier(t, ExactCopy, want)
			read(t, v, "read-source", "source.txt", true)
			input := []byte(wrong)
			d := v.Before(background, event(v, "bad"), Operation{Kind: "write", Path: "destination.txt", Content: input})
			if d.Allowed || d.Code != "content_mismatch" || string(input) != wrong {
				t.Fatal(d)
			}
			if _, err := os.Stat(filepath.Join(root, "destination.txt")); !os.IsNotExist(err) {
				t.Fatal("verifier wrote destination")
			}
			v.After(background, event(v, "bad"), Completion{Status: "blocked"})
			// A proven undispatched rejection allows a new correct proposal; no
			// action is automatically replayed or synthesized by the verifier.
			write(t, v, root, want)
			read(t, v, "read-back", "destination.txt", true)
			if r := v.Finish(background, "completed"); r.State != "passed" {
				t.Fatal(r)
			}
		})
	}
}

func TestSourceReadCannotBeSkippedOrTruncated(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		v, _ := testVerifier(t, ExactCopy, []byte("exact"))
		if truncated {
			read(t, v, "partial", "source.txt", false)
		}
		d := v.Before(background, event(v, "write"), Operation{Kind: "write", Path: "destination.txt", Content: []byte("exact")})
		if d.Allowed || d.Code != "complete_source_read_required" {
			t.Fatal(d)
		}
		if r := v.Finish(background, "completed"); r.State == "passed" {
			t.Fatal(r)
		}
	}
}

func TestHostPermissionRejectionIsNotUnknownMutation(t *testing.T) {
	v, root := testVerifier(t, ExactCopy, []byte("exact"))
	read(t, v, "read-source", "source.txt", true)
	id := event(v, "host-blocked")
	if d := v.Before(background, id, Operation{Kind: "write", Path: "destination.txt", Content: []byte("exact")}); !d.Allowed {
		t.Fatal(d)
	}
	if d := v.After(background, id, Completion{Status: "blocked"}); d.Allowed || d.Code != "host_rejected_before_dispatch" {
		t.Fatal(d)
	}
	if _, err := os.Stat(filepath.Join(root, "destination.txt")); !os.IsNotExist(err) {
		t.Fatal("blocked write executed")
	}
	// A later independently authorized proposal is possible, but the verifier
	// neither grants it nor automatically dispatches a retry.
	write(t, v, root, []byte("exact"))
	read(t, v, "read-back", "destination.txt", true)
	if r := v.Finish(background, "completed"); r.State != "passed" {
		t.Fatal(r)
	}
}

func TestFalseCompletionAndUnknownEffects(t *testing.T) {
	cases := []string{"no-tools", "missing-write-result", "false-write-success", "write-error", "write-unknown", "missing-readback", "partial-readback", "external-failed", "cancelled", "late-corruption", "changed-source", "unrelated-write", "missing-blocked-result"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			v, root := testVerifier(t, ExactCopy, []byte("exact"))
			if name == "no-tools" {
				if r := v.Finish(background, "completed"); r.State != "unverified" {
					t.Fatal(r)
				}
				return
			}
			read(t, v, "read-source", "source.txt", true)
			id := event(v, "write")
			if d := v.Before(background, id, Operation{Kind: "write", Path: "destination.txt", Content: []byte("exact")}); !d.Allowed {
				t.Fatal(d)
			}
			if name == "missing-write-result" {
				testFile(t, root, "destination.txt", []byte("exact"))
			} else {
				actual, status := "exact", "completed"
				if name == "false-write-success" {
					actual = "1|exact"
				}
				if name == "write-error" {
					status = "failed"
				}
				if name == "write-unknown" {
					status = "unknown"
				}
				testFile(t, root, "destination.txt", []byte(actual))
				v.After(background, id, Completion{Status: status})
				if status == "completed" && name != "false-write-success" && name != "missing-readback" {
					read(t, v, "read-back", "destination.txt", name != "partial-readback")
				}
			}
			switch name {
			case "late-corruption":
				testFile(t, root, "destination.txt", []byte("wrong"))
			case "changed-source":
				testFile(t, root, "source.txt", []byte("changed"))
			case "unrelated-write":
				testFile(t, root, "unexpected.txt", []byte("oops"))
			case "missing-blocked-result":
				v.Before(background, event(v, "again"), Operation{Kind: "write", Path: "destination.txt", Content: []byte("exact")})
			}
			exit := "completed"
			if name == "external-failed" {
				exit = "failed"
			}
			if name == "cancelled" {
				exit = "cancelled"
			}
			if r := v.Finish(background, exit); r.State == "passed" {
				t.Fatal("false completion", r)
			}
		})
	}
}

func TestNoChangesAndMissingHooks(t *testing.T) {
	for _, kind := range []string{"clean", "write-attempt", "changed", "deleted", "missing-hooks", "unconfirmed-read"} {
		t.Run(kind, func(t *testing.T) {
			v, root := testVerifier(t, NoChanges, []byte("original"))
			switch kind {
			case "write-attempt":
				if d := v.Before(background, event(v, "write"), Operation{Kind: "write", Path: "new.txt", Content: []byte("x")}); d.Allowed {
					t.Fatal(d)
				}
			case "changed":
				testFile(t, root, "source.txt", []byte("changed"))
			case "deleted":
				if err := os.Remove(filepath.Join(root, "source.txt")); err != nil {
					t.Fatal(err)
				}
			case "missing-hooks":
				v.session = "" // Simulate no supported handshake, not a model-provided claim.
			case "unconfirmed-read":
				v.Before(background, event(v, "read"), Operation{Kind: "read", Path: "source.txt"})
			}
			r := v.Finish(background, "completed")
			if (r.State == "passed") != (kind == "clean") {
				t.Fatal(r)
			}
		})
	}
}

func TestEventBindingAndReplay(t *testing.T) {
	for _, kind := range []string{"wrong-run", "wrong-session", "empty-turn", "empty-call", "duplicate-before", "duplicate-after", "after-without-before", "rebind", "rejected-dispatched"} {
		t.Run(kind, func(t *testing.T) {
			v, _ := testVerifier(t, NoChanges, []byte("exact"))
			id := event(v, "read")
			switch kind {
			case "wrong-run":
				id.Run = "other"
			case "wrong-session":
				id.Session = "child"
			case "empty-turn":
				id.Turn = ""
			case "empty-call":
				id.Call = ""
			}
			op := Operation{Kind: "read", Path: "source.txt"}
			switch kind {
			case "rebind":
				v.Bind(v.RunID(), "replacement")
			case "after-without-before":
				v.After(background, id, Completion{Status: "completed", CompleteRead: true})
			case "rejected-dispatched":
				v.Before(background, id, Operation{Kind: "write", Path: "source.txt"})
				v.After(background, id, Completion{Status: "completed"})
			default:
				v.Before(background, id, op)
				if kind == "duplicate-before" {
					v.Before(background, id, op)
				}
				if kind == "duplicate-after" {
					v.After(background, id, Completion{Status: "completed", CompleteRead: true})
					v.After(background, id, Completion{Status: "completed", CompleteRead: true})
				}
			}
			if r := v.Finish(background, "completed"); r.State != "failed" {
				t.Fatal(r)
			}
		})
	}
}

func TestCancellationConcurrencyAndPrivateReceipts(t *testing.T) {
	v, _ := testVerifier(t, NoChanges, []byte("PRIVATE-CONTENT-NOT-FOR-LOGS"))
	ctx, cancel := context.WithCancel(background)
	cancel()
	if d := v.Before(ctx, event(v, "read"), Operation{Kind: "read", Path: "source.txt"}); d.Allowed {
		t.Fatal(d)
	}
	var group sync.WaitGroup
	for i := range 20 {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			v.Before(background, event(v, fmt.Sprint(i)), Operation{Kind: "read", Path: "source.txt"})
			v.Finish(background, "completed")
		}(i)
	}
	group.Wait()
	r := v.Finish(background, "completed")
	if r.State != "cancelled" {
		t.Fatal(r)
	}
	data, err := json.Marshal(r)
	if err != nil || strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "source.txt") {
		t.Fatal("private evidence leaked")
	}
}
