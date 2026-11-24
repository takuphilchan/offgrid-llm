package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// All processes, grant files, account state and filesystem effects belong to
// this disposable fixture. No installed executable or live registry is used.
func TestStdioProtectionRealForcedWorkerCleanupDoesNotReplay(t *testing.T) {
	if !nativeCleanupVerified(nil) {
		t.Skip("native descendant cleanup unavailable")
	}
	base := protectionTestBase(t)
	t.Setenv("APPDATA", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	worker := filepath.Join(base, "offgrid-executor"+ext)
	helper := filepath.Join(base, "descendant"+ext)
	source := filepath.Join(base, "descendant.go")
	const program = `package main
import("os";"os/exec";"time")
func main(){
 if len(os.Args)==1 {c:=exec.Command(os.Args[0],"child");if c.Start()!=nil{os.Exit(2)};c.Wait();return}
 os.WriteFile("environment",[]byte(os.Getenv("OFFGRID_EXECUTOR_TOKEN")+os.Getenv("OFFGRID_EXECUTOR_PROTECTION_RECOVERY")),0600)
 f,e:=os.OpenFile("heartbeat",os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600);if e!=nil{os.Exit(3)};defer f.Close()
 os.WriteFile("child-started",[]byte("ready"),0600)
 for {f.Write([]byte("x"));f.Sync();time.Sleep(10*time.Millisecond)}
}
`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	for _, build := range [][]string{{"build", "-o", worker, "../../cmd/offgrid-executor"}, {"build", "-o", helper, source}} {
		command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"+ext), build...)
		if data, err := command.CombinedOutput(); err != nil {
			t.Fatal(string(data), err)
		}
	}
	bytes, err := os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(bytes)
	root := protectionTestDirectory(t, base, "workspace")
	state := filepath.Join(base, "journal")
	config := protectionTestConfig(root, "native-survivor")
	config.Permissions.Processes = true
	grant := filepath.Join(base, "grant.json")
	data, _ := json.Marshal(config)
	if err = os.WriteFile(grant, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	registry, err := AccountHostProtectionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	foreignRoot := protectionTestDirectory(t, base, "foreign")
	foreign, err := registry.ReserveExecution(ctx, protectionTestConfig(foreignRoot, "other-native-owner"), protectionTestDirectory(t, base, "foreign-journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.CancelUnactivated(context.Background())
	endpoint, err := StartStdio(ctx, worker, hex.EncodeToString(digest[:]), grant, state)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	if endpoint.protection == nil {
		t.Fatal("updated worker did not hand off private recovery authority")
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("live worker lost reservation", err)
	}
	args, _ := json.Marshal(ProcessSpec{Program: helper})
	request := Request{Version: ProtocolVersion, ID: "native-crash-process", OperationID: "native-crash-process", Binding: config.Binding, Operation: "process_start", Arguments: args, Approved: true}
	response, err := endpoint.Call(ctx, request)
	if err != nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal("native dispatch", response, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err = os.Stat(filepath.Join(root, "child-started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disposable descendant never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if data, err = os.ReadFile(filepath.Join(root, "environment")); err != nil || len(data) != 0 {
		t.Fatal("private startup input leaked into child", err)
	}
	if err = endpoint.stop(); err != nil {
		t.Fatal("owned force termination failed", err)
	}
	select {
	case <-endpoint.done:
	case <-ctx.Done():
		t.Fatal("native cleanup did not settle", ctx.Err())
	}
	if err = endpoint.Close(); err == nil {
		t.Fatal("forced worker exit was misreported as successful task completion")
	}
	if err = endpoint.RetryProtectionSettlement(ctx); err != nil {
		t.Fatal("surviving native owner could not settle", err)
	}
	info, err := os.Stat(filepath.Join(root, "heartbeat"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	later, err := os.Stat(filepath.Join(root, "heartbeat"))
	if err != nil || info.Size() != later.Size() {
		t.Fatal("descendant wrote after confirmed cleanup", err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, foreignRoot); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("survivor released another owner", err)
	}
	journal, err := OpenJournal(state)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := journal.Inspect(ctx, request)
	if err != nil || receipt.State != "uncertain" {
		t.Fatal("crashed effect was falsely completed", receipt, err)
	}
	if err = journal.Close(); err != nil {
		t.Fatal(err)
	}
	// An explicit matching restart can inspect/reconcile the prior operation;
	// its original ID must never start the helper again.
	restarted, err := StartStdio(ctx, worker, hex.EncodeToString(digest[:]), grant, state)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	response, err = restarted.Call(ctx, request)
	if err != nil || response.Receipt == nil || response.Receipt.State != "uncertain" {
		t.Fatal("restart replayed uncertain command", response, err)
	}
	time.Sleep(100 * time.Millisecond)
	later, err = os.Stat(filepath.Join(root, "heartbeat"))
	if err != nil || info.Size() != later.Size() {
		t.Fatal("restart repeated native effect", err)
	}
	if err = restarted.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); err != nil {
		t.Fatal("confirmed owner cleanup did not unblock browser", err)
	}
}
