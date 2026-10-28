package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestNativeApplicationPrivateHostIPCUsesCanonicalExecution(t *testing.T) {
	s, server, _, leaked := executionHTTPFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pack := buildExecutionPackFixture(t, ctx)
	client, err := serviceclient.New(server.URL, "user-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	var pair execution.Pairing
	if err = client.StrictJSON(ctx, http.MethodPost, "/api/v2/execution/pairing", struct{}{}, &pair, 4096); err != nil {
		t.Fatal(err)
	}
	root, state := t.TempDir(), filepath.Join(t.TempDir(), "native-state")
	file := "offgrid-executor"
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	command := exec.CommandContext(ctx, filepath.Join(pack, file), "--connect", "--state-dir", state)
	// Like native main, supply no user API key/cookie or generic command channel.
	command.Env = append(os.Environ(), "OFFGRID_API_KEY=", "OFFGRID_EXECUTOR_TOKEN=")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); _ = command.Process.Kill() }()
	if err = execution.WriteFrame(input, execution.NativeHostRequest{Protocol: 1, Service: server.URL, Pairing: pair, Folder: root, Permissions: execution.Permissions{Read: true, Write: true}}); err != nil {
		t.Fatal(err)
	}
	var ready execution.NativeHostState
	if err = execution.ReadFrame(output, &ready); err != nil {
		t.Fatal(err, diagnostics.String())
	}
	if ready.State != "ready" || ready.Resource == nil || ready.Resource.Root != root || ready.Resource.Permissions.Processes {
		t.Fatal(ready)
	}
	ref := ready.Resource.Reference
	if err = s.executionHosts.Admit(ctx, pair.Actor, pair.Workspace, "native-private-job", ref); err != nil {
		t.Fatal(err)
	}
	const exact = "Native IPC — café 日本語"
	args, _ := json.Marshal(map[string]any{"path": "native.txt", "data": []byte(exact)})
	result, err := s.executionHosts.Call(ctx, pair.Actor, pair.Workspace, "native-private-job", "write-once", ref, "file_write", args)
	if err != nil || result.Receipt == nil || result.Receipt.State != "complete" {
		t.Fatal(result, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "native.txt"))
	if err != nil || string(data) != exact {
		t.Fatal(string(data), err)
	}
	input.Close()
	var stopped execution.NativeHostState
	if err = execution.ReadFrame(output, &stopped); err != nil || stopped.State != "stopped" {
		t.Fatal(stopped, err, diagnostics.String())
	}
	if err = command.Wait(); err != nil {
		t.Fatal(err, diagnostics.String())
	}
	if got := s.executionHosts.List(ctx, pair.Actor, pair.Workspace); len(got) != 0 {
		t.Fatal("native lease survived EOF", got)
	}
	if leaked.Load() != 0 {
		t.Fatal("user credential reached host transport")
	}
}

func buildExecutionPackFixture(t *testing.T, ctx context.Context) string {
	t.Helper()
	directory := t.TempDir()
	file, gofile := "offgrid-executor", "go"
	if runtime.GOOS == "windows" {
		file += ".exe"
		gofile += ".exe"
	}
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", gofile), "build", "-o", filepath.Join(directory, file), "./cmd/offgrid-executor")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(string(output), err)
	}
	data, err := os.ReadFile(filepath.Join(directory, file))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	pack := execution.Pack{Schema: 1, Protocol: 1, Revision: execution.PackRevision, Version: "development", Platform: runtime.GOOS, Architecture: runtime.GOARCH, File: file, SHA256: hex.EncodeToString(digest[:])}
	encoded, _ := json.Marshal(pack)
	if err = os.WriteFile(filepath.Join(directory, "execution-pack.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return directory
}

type executionSpyTransport struct {
	next  http.RoundTripper
	pairs int
	scope execution.Context
	token string
}

func (s *executionSpyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/api/v2/execution/pairing" {
		s.pairs++
	}
	if r.URL.Path == "/api/v2/execution/host/enroll" && r.GetBody != nil {
		body, _ := r.GetBody()
		var value struct {
			Token   string
			Context execution.Context
		}
		_ = json.NewDecoder(body).Decode(&value)
		body.Close()
		s.scope, s.token = value.Context, value.Token
	}
	return s.next.RoundTrip(r)
}

func TestNativeApprovalStartsPinnedWorkerOverAuthenticatedBridge(t *testing.T) {
	s, server, _, leaked := executionHTTPFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pack := buildExecutionPackFixture(t, ctx)
	spy := &executionSpyTransport{next: server.Client().Transport}
	client, err := serviceclient.New(server.URL, "user-secret", &http.Client{Transport: spy})
	if err != nil {
		t.Fatal(err)
	}
	root, state := t.TempDir(), filepath.Join(t.TempDir(), "private-state")
	options := execution.LocalHostOptions{Folder: root, PackDirectory: pack, StateDirectory: state, Version: "development", Permissions: execution.Permissions{Read: true, Write: true, Processes: true}}
	if _, err = execution.StartLocalHost(ctx, client, options, func(execution.LocalApproval) bool { return false }); err == nil {
		t.Fatal("decline created authority")
	}
	if spy.pairs != 0 {
		t.Fatal("decline requested pairing")
	}
	if _, err = os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("decline created state", err)
	}
	approvals := 0
	host, err := execution.StartLocalHost(ctx, client, options, func(a execution.LocalApproval) bool {
		approvals++
		if a.Root != root || a.Service != server.URL || a.Backend != "local" || a.Permissions != options.Permissions {
			t.Error("local scope mismatch", a)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Error(err)
		}
	})
	if approvals != 1 || spy.pairs != 1 || leaked.Load() != 0 {
		t.Fatal(approvals, spy.pairs, leaked.Load())
	}
	if spy.token == "" || strings.Contains(spy.token, strings.TrimPrefix(spy.scope.Binding.Grant, "grant-")) || strings.Contains(spy.token, strings.TrimPrefix(spy.scope.Binding.Session, "executor-")) {
		t.Fatal("host secret missing or derived from public identity")
	}
	ref := host.Resource().Reference
	if err = s.executionHosts.Admit(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, "pinned-job", ref); err != nil {
		t.Fatal(err)
	}
	const exact = "1| genuine content\nOffGrid — café 日本語 12345"
	args, _ := json.Marshal(map[string]any{"path": "exact.txt", "data": []byte(exact)})
	response, err := s.executionHosts.Call(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, "pinned-job", "create-once", ref, "file_write", args)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	if actual, err := os.ReadFile(filepath.Join(root, "exact.txt")); err != nil || string(actual) != exact {
		t.Fatal(string(actual), err)
	}
	// The routine test uses the same owned worker and approved folder, not a
	// container directory or an independent CLI executor. It imports only stdlib.
	for name, content := range map[string]string{
		"go.mod":          "module execution-fixture\n\ngo 1.26\n",
		"fixture_test.go": "package fixture\nimport \"testing\"\nfunc TestExact(t *testing.T) { if \"café 日本語\" == \"\" { t.Fatal(\"bad\") } }\n",
	} {
		if err = os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	processArgs, _ := json.Marshal(execution.ProcessSpec{Program: "go", Arguments: []string{"test", "."}, TimeoutSeconds: 45})
	response, err = s.executionHosts.Call(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, "pinned-job", "routine-test-once", ref, "process_start", processArgs)
	if err != nil || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal(response, err)
	}
	var process execution.ProcessResult
	if err = json.Unmarshal(response.Receipt.Result, &process); err != nil || process.Handle == "" {
		t.Fatal(process, err)
	}
	for process.State == "running" {
		waitArgs, _ := json.Marshal(map[string]any{"handle": process.Handle, "wait_ms": 1000})
		response, err = s.executionHosts.Call(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, "pinned-job", "wait-result", ref, "process_wait", waitArgs)
		if err != nil || response.Receipt == nil || json.Unmarshal(response.Receipt.Result, &process) != nil {
			t.Fatal(response, err)
		}
	}
	if process.State != "exited" || process.ExitCode == nil || *process.ExitCode != 0 || !strings.Contains(string(process.Output), "ok") {
		t.Fatal("routine test did not pass", process, string(process.Output))
	}
	// A changed payload under the original operation identity must never start a
	// second command, even after the first command completed successfully.
	changed, _ := json.Marshal(execution.ProcessSpec{Program: "go", Arguments: []string{"test", "./..."}})
	response, err = s.executionHosts.Call(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, "pinned-job", "routine-test-once", ref, "process_start", changed)
	if err != nil || response.Error != execution.ErrConflict.Error() {
		t.Fatal("conflicting command identity accepted", response, err)
	}
	binding := host.Context.Binding
	binding.Job = "pinned-job"
	original, err := execution.NewIntent(binding, "create-once", "file_write", args, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "exact.txt"), []byte("manual change"), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := s.executionHosts.InspectReceipt(ctx, binding.Actor, binding.Workspace, ref, original)
	if err != nil || saved.Receipt == nil || saved.Receipt.State != "complete" {
		t.Fatal(saved, err)
	}
	if actual, _ := os.ReadFile(filepath.Join(root, "exact.txt")); string(actual) != "manual change" {
		t.Fatal("private receipt query replayed edit")
	}
	if err = host.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.executionHosts.Resolve(ctx, host.Context.Binding.Actor, host.Context.Binding.Workspace, ref); err == nil {
		t.Fatal("closed host still ready")
	}
}
