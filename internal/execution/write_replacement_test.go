package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestJournaledWritePreservesConcurrentEdits(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("replacement backend unsupported")
	}
	for _, fault := range []string{"none", "late-edit", "post-edit", "lost-ack", "receipt-failure"} {
		t.Run(fault, func(t *testing.T) {
			w, b, root := workerFixture(t)
			put(t, root, "input.txt", "before")
			ctx := context.Background()
			r := request(w, b, "file_write", "replace", map[string]any{"path": "input.txt", "data": []byte("proposed"), "revision": digest([]byte("before"))})
			if _, fresh, err := w.Journal.Claim(ctx, r.OperationID, r.Payload()); err != nil || !fresh {
				t.Fatal(err)
			}
			if fault == "receipt-failure" {
				if _, err := w.Journal.db.Exec(`CREATE TRIGGER fail_replacement_receipt BEFORE UPDATE ON execution_replacements WHEN NEW.state!='dispatching' BEGIN SELECT RAISE(FAIL,'injected receipt failure'); END;`); err != nil {
					t.Fatal(err)
				}
			}
			dispatch := func(to *replacementDirectory, name string, from *replacementDirectory, staged, backup string) (string, error) {
				if fault == "late-edit" {
					put(t, root, "input.txt", "user edit after precheck")
				}
				saved, err := replacePinned(to, name, from, staged, backup)
				if err != nil {
					return saved, err
				}
				if fault == "post-edit" {
					put(t, root, "input.txt", "newer user edit")
				}
				if fault == "lost-ack" {
					return saved, errors.New("lost acknowledgement")
				}
				return saved, nil
			}
			result, err := w.Journal.replaceFile(ctx, w.Session, b, r.OperationID, "input.txt", []byte("proposed"), digest([]byte("before")), 0600, dispatch)
			if fault == "none" {
				if err != nil || !result.Verified {
					t.Fatal(result, err)
				}
			} else {
				var mutation *MutationError
				if !errors.As(err, &mutation) {
					t.Fatal("lost mutation uncertainty", result, err)
				}
			}
			if fault == "receipt-failure" && !w.Session.revoked.Load() {
				t.Fatal("receipt loss did not revoke dispatch")
			}
			if result.ReplacementID == "" {
				t.Fatal("missing recovery identity")
			}
			private := filepath.Join(w.Journal.directory, "replacements", result.ReplacementID)
			proposed, readErr := os.ReadFile(filepath.Join(private, "proposed"))
			if readErr != nil || string(proposed) != "proposed" {
				t.Fatal("proposed version lost", readErr)
			}
			oldName := "displaced"
			if runtime.GOOS == "linux" {
				oldName = "staged"
			}
			old, readErr := os.ReadFile(filepath.Join(private, oldName))
			wantOld := "before"
			if fault == "late-edit" {
				wantOld = "user edit after precheck"
			}
			if readErr != nil || string(old) != wantOld {
				t.Fatal("actual displaced version lost", string(old), readErr)
			}
			want := "proposed"
			if fault == "post-edit" {
				want = "newer user edit"
			}
			current, readErr := os.ReadFile(filepath.Join(root, "input.txt"))
			if readErr != nil || string(current) != want {
				t.Fatal("newer user edit rolled back", readErr)
			}
			// A fresh observation of a durable outstanding claim cannot repeat it.
			if _, fresh, err := w.Journal.Claim(ctx, r.OperationID, r.Payload()); err != nil || fresh {
				t.Fatal("uncertain write replayed", err)
			}
		})
	}
}

func TestWorkerReplacementReceiptAndRetry(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("replacement backend unsupported")
	}
	w, b, root := workerFixture(t)
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, root, "nested/input.txt", "before")
	r := request(w, b, "file_write", "replace-once", map[string]any{"path": "nested/input.txt", "data": []byte("proposed"), "revision": digest([]byte("before"))})
	response := w.Handle(context.Background(), r)
	var result WriteResult
	if response.Receipt == nil || response.Receipt.State != "complete" || json.Unmarshal(response.Receipt.Result, &result) != nil || !result.Verified || !result.OriginalRetained {
		t.Fatal(response)
	}
	put(t, root, "nested/input.txt", "user edit after completion")
	repeated := w.Handle(context.Background(), r)
	if repeated.Receipt == nil || string(repeated.Receipt.Result) != string(response.Receipt.Result) {
		t.Fatal("retry changed receipt", repeated)
	}
	data, _ := os.ReadFile(filepath.Join(root, "nested/input.txt"))
	if string(data) != "user edit after completion" {
		t.Fatal("retried a completed write")
	}
}

func TestDirectWriteCrashHelper(t *testing.T) {
	base := os.Getenv("OFFGRID_WRITE_CRASH_FIXTURE")
	if base == "" {
		t.Skip("subprocess fixture only")
	}
	root, private := filepath.Join(base, "root"), filepath.Join(base, "private")
	b := Binding{Actor: "actor", Workspace: "workspace", Host: "host", Session: "session", Grant: "grant", Revision: 1, Job: "job"}
	s, err := OpenSession(Context{Version: 1, Binding: b, Backend: "local", Root: root, Permissions: Permissions{Read: true, Write: true}})
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(private)
	if err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := j.Claim(context.Background(), "crash", []byte("crash payload")); err != nil || !fresh {
		t.Fatal(err)
	}
	dispatch := func(to *replacementDirectory, name string, from *replacementDirectory, staged, backup string) (string, error) {
		_, err := replacePinned(to, name, from, staged, backup)
		if err != nil {
			t.Fatal(err)
		}
		os.Exit(79)
		return "", nil
	}
	_, err = j.replaceFile(context.Background(), s, b, "crash", "input.txt", []byte("proposed"), digest([]byte("before")), 0600, dispatch)
	t.Fatal("fixture did not exit at dispatch", err)
}

func TestDirectWriteCrashRetainsVersionsAndRejectsReplay(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("replacement backend unsupported")
	}
	base := t.TempDir()
	for _, name := range []string{"root", "private"} {
		if err := os.Mkdir(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(base, "root"), "input.txt", "before")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestDirectWriteCrashHelper$")
	child.Env = append(os.Environ(), "OFFGRID_WRITE_CRASH_FIXTURE="+base)
	output, err := child.CombinedOutput()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 79 {
		t.Fatal(string(output), err)
	}
	j, err := OpenJournal(filepath.Join(base, "private"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	receipt, fresh, err := j.Claim(context.Background(), "crash", []byte("crash payload"))
	if err != nil || fresh || receipt.State != "uncertain" {
		t.Fatal(receipt, fresh, err)
	}
	var directory, state string
	if err = j.db.QueryRow(`SELECT directory,state FROM execution_replacements WHERE operation='crash'`).Scan(&directory, &state); err != nil || state != "dispatching" {
		t.Fatal(state, err)
	}
	for _, file := range []string{"proposed", "staged", "displaced"} {
		data, readErr := os.ReadFile(filepath.Join(base, "private", "replacements", directory, file))
		if readErr == nil && string(data) != "before" && string(data) != "proposed" {
			t.Fatal("corrupted retained version", file)
		}
	}
	oldName := "displaced"
	if runtime.GOOS == "linux" {
		oldName = "staged"
	}
	old, err := os.ReadFile(filepath.Join(base, "private", "replacements", directory, oldName))
	if err != nil || string(old) != "before" {
		t.Fatal("crash lost displaced file", err)
	}
	proposed, err := os.ReadFile(filepath.Join(base, "private", "replacements", directory, "proposed"))
	if err != nil || string(proposed) != "proposed" {
		t.Fatal("crash lost proposed file", err)
	}
	put(t, filepath.Join(base, "root"), "input.txt", "newer user edit")
	if _, fresh, err = j.Claim(context.Background(), "crash", []byte("crash payload")); err != nil || fresh {
		t.Fatal("restart replayed write", err)
	}
}
