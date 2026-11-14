package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type integrationCrashFixtureState struct {
	Store   string
	Source  string
	Policy  WorkingCopyPolicy
	Request IntegrateChangeSetRequest
}

// Side effects are confined to the parent test's temporary directory. Exit
// bypasses all defers, SQLite closes, and ownership/session cleanup on purpose.
func TestIntegrationCrashProcessFixture(t *testing.T) {
	i := slices.Index(os.Args, "offgrid-integration-crash")
	if i < 0 {
		return
	}
	if len(os.Args) != i+3 {
		os.Exit(78)
	}
	crashAt, err := strconv.Atoi(os.Args[i+2])
	if err != nil {
		os.Exit(78)
	}
	e, store, _, source, _, req := integrationFixture(t)
	state := integrationCrashFixtureState{Store: store.directory, Source: source, Policy: store.policy, Request: req}
	data, _ := json.Marshal(state)
	if err = os.WriteFile(os.Args[i+1], data, 0600); err != nil {
		os.Exit(78)
	}
	_, err = store.integrateChangeSet(context.Background(), e.Worker.Session, req, &integrationHooks{afterDispatch: func(ordinal int) error {
		if ordinal == crashAt {
			os.Exit(77)
		}
		return nil
	}})
	t.Fatalf("expected process exit after dispatch %d: %v", crashAt, err)
}

func TestIntegrationRecoversRealProcessExitAfterEachFile(t *testing.T) {
	for crashAt := 0; crashAt < 5; crashAt++ {
		t.Run(strconv.Itoa(crashAt), func(t *testing.T) {
			owned := t.TempDir()
			statePath := filepath.Join(owned, "fixture.json")
			cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrationCrashProcessFixture$", "--", "offgrid-integration-crash", statePath, strconv.Itoa(crashAt))
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if !strings.EqualFold(key, "TEMP") && !strings.EqualFold(key, "TMP") && !strings.EqualFold(key, "TMPDIR") && !strings.EqualFold(key, "GOTMPDIR") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			// Go 1.26 testing.TempDir prefers GOTMPDIR over the OS temporary
			// directory. Pin both so inherited build settings cannot escape.
			cmd.Env = append(cmd.Env, "TEMP="+owned, "TMP="+owned, "TMPDIR="+owned, "GOTMPDIR="+owned)
			output, err := cmd.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 77 {
				t.Fatalf("crash helper failed: %v %s", err, output)
			}
			data, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			var state integrationCrashFixtureState
			if err = json.Unmarshal(data, &state); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{state.Store, state.Source} {
				rel, err := filepath.Rel(owned, name)
				if err != nil || !filepath.IsLocal(rel) {
					t.Fatal("helper escaped owned temporary state", name, err)
				}
			}
			store, err := OpenWorkingCopies(state.Store, state.Policy)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			id, _ := integrationIdentity(state.Request)
			page, err := store.InspectIntegration(context.Background(), id, state.Request.Source, 0, 64)
			if err != nil || page.State != "uncertain" || page.Integrated != crashAt || page.Files[crashAt].State != "uncertain" || !page.Files[crashAt].Dispatched {
				t.Fatal("real crash lost effects", page, err)
			}
			for _, file := range page.Files[crashAt+1:] {
				if file.State != "pending" || file.Dispatched {
					t.Fatal("pending file replayed", file)
				}
			}
			// Explicit test-only authority for this owned folder. Recovery itself
			// never restores a host grant or resumes a command.
			session, err := OpenSession(Context{Version: 1, Binding: state.Request.Source, Backend: "local", Root: state.Source, Directory: ".", Permissions: Permissions{Read: true, Write: true}})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			before, err := inventoryWorkingCopy(context.Background(), session.root, WorkingCopyRequest{Parent: state.Request.Source, MaxBytes: state.Policy.MaxCopyBytes}, state.Policy)
			if err != nil {
				t.Fatal(err)
			}
			again, err := store.IntegrateChangeSet(context.Background(), session, state.Request)
			if err != nil || again.State != "uncertain" || again.ID != id {
				t.Fatal("restart retry lost uncertain attempt", again, err)
			}
			after, err := inventoryWorkingCopy(context.Background(), session.root, WorkingCopyRequest{Parent: state.Request.Source, MaxBytes: state.Policy.MaxCopyBytes}, state.Policy)
			if err != nil || !reflect.DeepEqual(before.Entries, after.Entries) {
				t.Fatal("restart retry mutated files", err)
			}
		})
	}
}
