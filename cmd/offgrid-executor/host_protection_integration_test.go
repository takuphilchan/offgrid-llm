package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/testhost"
)

// Both hosts share only the account namespace. In particular, the browser does
// not receive the executor's workspace, and the executor has no desktop profile.
func TestIndependentHostsProtectCustomBrowserProfile(t *testing.T) {
	testhost.Account(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	binary := filepath.Join(testhost.PrivateDirectory(t), "offgrid-computer")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../offgrid-computer")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native storage checker: %v: %s", err, output)
	}
	environment := func() []string {
		result := []string{}
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(strings.ToUpper(entry), "OFFGRID_DESKTOP_HOME=") {
				result = append(result, entry)
			}
		}
		return result
	}
	prepare := func(t *testing.T, directory string, accepted bool) {
		t.Helper()
		var frame bytes.Buffer
		if err := execution.WriteFrame(&frame, map[string]any{"protocol": 1, "kind": "prepare_browser_storage", "workspace_roots": []string{}}); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, binary, "--browser-storage", "--state", directory)
		command.Env, command.Stdin = environment(), &frame
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if !accepted {
			var denied struct {
				Protocol int             `json:"protocol"`
				ID       string          `json:"id"`
				Error    string          `json:"error"`
				Result   json.RawMessage `json:"result"`
			}
			reader := bytes.NewReader(output)
			decodeErr := computer.ReadControlFrame(reader, &denied)
			if err == nil || decodeErr != nil || reader.Len() != 0 || denied.ID != "browser-storage" || denied.Error != "computer_storage_unavailable" || string(denied.Result) != "null" {
				t.Fatalf("overlapping browser admitted: %v, output=%q, stderr=%s", err, output, stderr.String())
			}
			return
		}
		if err != nil {
			t.Fatalf("browser rejected: %v: %s", err, stderr.String())
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
		if err := computer.ReadControlFrame(reader, &reply); err != nil || reader.Len() != 0 || reply.ID != "browser-storage" || !reply.Result.Ready {
			t.Fatalf("invalid readiness: %+v: %v", reply, err)
		}
	}
	startExecutor := func(t *testing.T, root string, accepted bool) (*exec.Cmd, io.WriteCloser) {
		t.Helper()
		grant := filepath.Join(testhost.PrivateDirectory(t), "grant.json")
		config := execution.Context{Version: 1, Binding: execution.Binding{Actor: "fixture", Workspace: "workspace", Host: "host", Session: filepath.Base(filepath.Dir(grant)), Grant: "grant", Job: "job", Revision: 1}, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(grant, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := execution.ProtectPrivatePath(grant); err != nil {
			t.Fatal(err)
		}
		state := filepath.Join(testhost.PrivateDirectory(t), "journal")
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecutorSubprocess$", "--", "--stdio", "--grant-file", grant, "--state-dir", state)
		command.Env = append(environment(), "OFFGRID_EXECUTOR_TEST_CHILD=1", "OFFGRID_EXECUTOR_TOKEN="+strings.Repeat("a", 64))
		input, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := command.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { input.Close(); command.Process.Kill() })
		var reply execution.Response
		err = execution.ReadFrame(output, &reply)
		if !accepted {
			input.Close()
			if waitErr := command.Wait(); err == nil || waitErr == nil || !strings.Contains(stderr.String(), "host_protection_conflict") {
				t.Fatalf("overlapping executor admitted: read=%v, reply=%+v, wait=%v, stderr=%s", err, reply, waitErr, stderr.String())
			}
			return nil, nil
		}
		if err != nil || reply.ID != "startup" || reply.Error != "" {
			input.Close()
			command.Wait()
			t.Fatalf("executor startup: %+v, %v, stderr=%s", reply, err, stderr.String())
		}
		return command, input
	}

	t.Run("browser first excludes independent CLI", func(t *testing.T) {
		testhost.Account(t)
		parent := testhost.PrivateDirectory(t)
		profile := filepath.Join(parent, "custom-profile")
		prepare(t, profile, true)
		startExecutor(t, profile, false)
		startExecutor(t, parent, false)
	})
	t.Run("executor first excludes browser until confirmed close", func(t *testing.T) {
		testhost.Account(t)
		root := testhost.PrivateDirectory(t)
		profile := filepath.Join(root, "another-profile")
		command, input := startExecutor(t, root, true)
		prepare(t, profile, false)
		if _, err := os.Stat(profile); !os.IsNotExist(err) {
			t.Fatalf("denied browser mutated profile: %v", err)
		}
		input.Close()
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
		prepare(t, profile, true)
	})
}
