//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/testhost"
)

func TestExecutorFailedIsolatedRecoveryRetainsProtection(t *testing.T) {
	testhost.Account(t)
	t.Setenv("OFFGRID_EXECUTOR_TOKEN", strings.Repeat("a", 64))
	t.Setenv("OFFGRID_EXECUTOR_PROCESS_RUNTIME", "")
	root, private := testhost.PrivateDirectory(t), testhost.PrivateDirectory(t)
	state := filepath.Join(private, "state")
	if err := execution.EnsurePrivateDirectory(state); err != nil {
		t.Fatal(err)
	}
	config := execution.Context{
		Version: execution.ProtocolVersion,
		Binding: execution.Binding{Actor: "fixture", Workspace: "workspace", Host: "host", Session: "recover-prior", Grant: "grant", Revision: 1, Job: execution.BootstrapJob},
		Backend: "isolated_linux", Root: root, Directory: ".",
		Permissions: execution.Permissions{Read: true, Write: true, Processes: true},
		Isolation:   execution.IsolationPolicy{Socket: filepath.Join(private, "unavailable-docker.sock"), Image: "sha256:" + strings.Repeat("b", 64), Network: "none", MemoryBytes: 128 << 20, CPUMillis: 100, Pids: 16, UID: 1000, GID: 1000},
	}
	if _, err := os.Lstat(config.Isolation.Socket); !os.IsNotExist(err) {
		t.Fatalf("fixture socket must be absent: %v", err)
	}
	journal, err := execution.OpenJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = journal.Close(); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(state, "execution-dispatch.sqlite")
	db, err := storage.OpenSQLite(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	owner := strings.Repeat("c", 64)
	_, seedErr := db.Exec("INSERT INTO execution_operations(id,digest,state) VALUES('prior-effect',?,'dispatched')", strings.Repeat("d", 64))
	if seedErr == nil {
		_, seedErr = db.Exec("INSERT INTO execution_containers(operation,daemon,name,owner,container) VALUES('prior-effect','fixture-daemon',?,?,'')", "offgrid-"+owner, owner)
	}
	if err = errors.Join(seedErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	grant := filepath.Join(private, "grant.json")
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(grant, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = execution.ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	args := []string{"--stdio", "--grant-file", grant, "--state-dir", state}
	if err = run(args); !errors.Is(err, execution.ErrHostUnavailable) {
		t.Fatalf("did not reach unavailable isolated recovery: %v", err)
	}
	registry, err := execution.AccountHostProtectionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, path := range []string{filepath.Join(root, "browser"), filepath.Join(state, "browser")} {
		if _, err = registry.RegisterBrowserRoot(ctx, path); !errors.Is(err, execution.ErrHostProtectionConflict) {
			t.Fatalf("failed recovery released protected path %q: %v", path, err)
		}
	}
	if err = run(args); !errors.Is(err, execution.ErrHostProtectionUncertain) {
		t.Fatalf("retry adopted uncertain prior reservation: %v", err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, testhost.PrivateDirectory(t)); err != nil {
		t.Fatalf("unrelated browser admission was fenced: %v", err)
	}
	db, err = storage.OpenSQLite(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var operationState, retainedOwner, retainedName, retainedContainer string
	if err = db.QueryRow("SELECT state FROM execution_operations WHERE id='prior-effect'").Scan(&operationState); err != nil || operationState != "uncertain" {
		t.Fatalf("prior effect was replayed or lost: %q %v", operationState, err)
	}
	if err = db.QueryRow("SELECT owner,name,container FROM execution_containers WHERE operation='prior-effect'").Scan(&retainedOwner, &retainedName, &retainedContainer); err != nil || retainedOwner != owner || retainedName != "offgrid-"+owner || retainedContainer != "" {
		t.Fatalf("owned recovery evidence was changed: %q %q %q %v", retainedOwner, retainedName, retainedContainer, err)
	}
}
