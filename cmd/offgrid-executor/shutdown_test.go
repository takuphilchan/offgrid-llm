package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/testhost"
)

// Exercise the actual pinned worker, including a separately spawned child.
// Immediate supervisor kill used to leave that child alive and its receipt
// dispatched. All effects are restricted to this test's disposable directory.
func testEndpointCommandShutdown(t *testing.T, binary, digest string) {
	root, private := t.TempDir(), testhost.PrivateDirectory(t)
	grant, state := filepath.Join(private, "grant.json"), filepath.Join(private, "journal")
	b := execution.Binding{Actor: "fixture", Workspace: "workspace", Host: "host", Session: "shutdown-session", Grant: "grant", Revision: 1, Job: "supervisor"}
	config := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true, Processes: true}}
	encoded, _ := json.Marshal(config)
	if err := os.WriteFile(grant, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := execution.ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	source := `package main
import("os";"os/exec";"time")
func main(){
 if len(os.Args)>1 && os.Args[1]=="child" {
  if os.WriteFile("started",[]byte("ready"),0600)!=nil {os.Exit(2)}
  time.Sleep(3*time.Second)
  if os.WriteFile("late",[]byte("not stopped"),0600)!=nil {os.Exit(3)}
  return
 }
 self,err:=os.Executable();if err!=nil {os.Exit(4)}
 child:=exec.Command(self,"child");if child.Run()!=nil {os.Exit(5)}
}
`
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	// Compile before the shutdown deadline, using the test compiler environment.
	// The owned command still spawns a real descendant, but cold compilation is
	// no longer mistaken for a command that failed to start inside ten seconds.
	fixture := filepath.Join(root, "shutdown-fixture"+ext)
	buildCtx, buildCancel := context.WithTimeout(context.Background(), time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, filepath.Join(runtime.GOROOT(), "bin", "go"+ext), "build", "-o", fixture, filepath.Join(root, "fixture.go"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build shutdown fixture: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	endpoint, err := execution.StartStdio(ctx, binary, digest, grant, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { endpoint.Close() })
	args, _ := json.Marshal(execution.ProcessSpec{Program: fixture})
	r := execution.Request{Version: 1, ID: "start-fixture", OperationID: "start-fixture", Binding: b, Operation: "process_start", Arguments: args, Approved: true}
	response, err := endpoint.Call(ctx, r)
	if err != nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal(response, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture command did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	closeErr := endpoint.Close()
	if runtime.GOOS == "darwin" {
		var exitErr *exec.ExitError
		if !errors.As(closeErr, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatal("Darwin worker did not report its unsettled shutdown", closeErr)
		}
	} else if closeErr != nil {
		t.Fatal("worker failed to settle its command", closeErr)
	}
	journal, err := execution.OpenJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	receipt, err := journal.Inspect(context.Background(), r)
	var process execution.ProcessResult
	if runtime.GOOS == "darwin" {
		if err != nil || receipt.State != "uncertain" || json.Unmarshal(receipt.Result, &process) != nil || process.State != "cancelled" || process.ExitCode == nil || execution.ProcessDescendantsSettled(process) || execution.ProcessDescendantState(process) != "unknown" {
			t.Fatal("shutdown lost leader outcome or invented descendant retirement", receipt, process, err)
		}
		if _, err := os.Stat(filepath.Join(root, "late")); !os.IsNotExist(err) {
			t.Fatal("owned command continued after shutdown", err)
		}
		return
	}
	if err != nil || receipt.State != "complete" || json.Unmarshal(receipt.Result, &process) != nil || process.State != "cancelled" || process.ExitCode == nil || !execution.ProcessDescendantsSettled(process) {
		t.Fatal("shutdown did not persist observed command termination", receipt, process, err)
	}
	if _, err := os.Stat(filepath.Join(root, "late")); !os.IsNotExist(err) {
		t.Fatal("command continued after confirmed shutdown", err)
	}
}
