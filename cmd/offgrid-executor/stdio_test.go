package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestExecutorSubprocess(t *testing.T) {
	if os.Getenv("OFFGRID_EXECUTOR_TEST_CHILD") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			if err := run(os.Args[i+1:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestPrivateStdioListsOnlyApprovedFolder(t *testing.T) {
	root := t.TempDir()
	grant := filepath.Join(t.TempDir(), "grant.json")
	b := execution.Binding{Actor: "fixture", Workspace: "fixture-workspace", Host: "fixture-host", Session: "fixture-session", Grant: "fixture-grant", Job: "fixture-job", Revision: 1}
	config := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(grant, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := execution.ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "only-fixture.txt"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestExecutorSubprocess$", "--", "--stdio", "--grant-file", grant, "--state-dir", filepath.Join(t.TempDir(), "journal"))
	token := strings.Repeat("c", 64)
	command.Env = append(os.Environ(), "OFFGRID_EXECUTOR_TEST_CHILD=1", "OFFGRID_EXECUTOR_TOKEN="+token)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close(); command.Process.Kill() })
	var response execution.Response
	if err = execution.ReadFrame(output, &response); err != nil || response.ID != "startup" {
		t.Fatal(response, err)
	}
	request := execution.Request{Version: 1, ID: "request", OperationID: "operation", Binding: b, Token: token, Operation: "file_list", Arguments: json.RawMessage(`{"path":"."}`)}
	if err = execution.WriteFrame(input, request); err != nil {
		t.Fatal(err)
	}
	if err = execution.ReadFrame(output, &response); err != nil || response.Error != "" || response.Receipt == nil || !strings.Contains(string(response.Receipt.Result), "only-fixture.txt") {
		t.Fatal(response, err)
	}
	request.ID = "cross-actor"
	request.Binding.Actor = "someone-else"
	if err = execution.WriteFrame(input, request); err != nil {
		t.Fatal(err)
	}
	if err = execution.ReadFrame(output, &response); err != nil || response.Error != "execution_authority_denied" {
		t.Fatal(response, err)
	}
	// New canonical jobs require a supervisor admission, not a model tool call.
	request.Binding = b
	request.Binding.Job = "another-job"
	request.ID = "admit"
	request.Operation = "supervisor_admit"
	request.Arguments = json.RawMessage(`{}`)
	if err = execution.WriteFrame(input, request); err != nil {
		t.Fatal(err)
	}
	if err = execution.ReadFrame(output, &response); err != nil || response.Error != "" {
		t.Fatal(response, err)
	}
	request.ID = "new-job-list"
	request.Operation = "file_list"
	if err = execution.WriteFrame(input, request); err != nil {
		t.Fatal(err)
	}
	if err = execution.ReadFrame(output, &response); err != nil || response.Error != "" || response.Receipt == nil || !strings.Contains(string(response.Receipt.Result), "only-fixture.txt") {
		t.Fatal(response, err)
	}
	input.Close()
	if err = command.Wait(); err != nil {
		t.Fatal(err)
	}
}
