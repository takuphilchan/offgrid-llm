package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestReviewedLogProcessSpecifications(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".offgrid", "logs", "server.log")
	for _, test := range []struct {
		request cli.CommandRequest
		program string
		args    []string
	}{
		{cli.CommandRequest{Capability: "systemd-status"}, "systemctl", []string{"is-active", "offgrid-llm"}},
		{cli.CommandRequest{Capability: "llama-systemd-active"}, "systemctl", []string{"is-active", "llama-server"}},
		{cli.CommandRequest{Capability: "llama-systemd-status"}, "systemctl", []string{"status", "llama-server"}},
		{cli.CommandRequest{Capability: "llama-systemd-restart"}, "sudo", []string{"systemctl", "restart", "llama-server"}},
		{cli.CommandRequest{Capability: "llama-process-status"}, "pgrep", []string{"-x", "llama-server"}},
		{cli.CommandRequest{Capability: "llama-active-model"}, "sudo", []string{"tee", config.GetDefaultPaths().ActiveModel}},
		{cli.CommandRequest{Capability: "journalctl-logs", Args: []string{"50", "true"}}, "journalctl", []string{"-u", "offgrid-llm", "-f", "-n", "50"}},
		{cli.CommandRequest{Capability: "tail-logs", Args: []string{path, "12", "false"}}, "tail", []string{"-n", "12", path}},
	} {
		program, args, err := reviewedProcessSpec(test.request)
		if err != nil || program != test.program || !reflect.DeepEqual(args, test.args) {
			t.Fatalf("%#v => %s %#v %v", test.request, program, args, err)
		}
	}
	for _, request := range []cli.CommandRequest{
		{Capability: "unknown"},
		{Capability: "systemd-status", Args: []string{"other-service"}},
		{Capability: "llama-systemd-restart", Args: []string{"other-service"}},
		{Capability: "journalctl-logs", Args: []string{"-1", "false"}},
		{Capability: "journalctl-logs", Args: []string{"5", "maybe"}},
		{Capability: "tail-logs", Args: []string{"C:/private/log", "5", "false"}},
		{Capability: "tail-logs", Args: []string{path, "999999", "false"}},
	} {
		if _, _, err := reviewedProcessSpec(request); err == nil {
			t.Fatalf("accepted unreviewed request: %#v", request)
		}
	}
}

type fakeLlamaReloadRunner struct{ requests []string }

func (runner *fakeLlamaReloadRunner) Run(_ context.Context, request cli.CommandRequest) error {
	runner.requests = append(runner.requests, request.Capability)
	if request.Capability == "llama-systemd-active" {
		return errors.New("inactive")
	}
	return nil
}

type fakeWorkspaceProcess struct{ requests []cli.CommandRequest }

func (runner *fakeWorkspaceProcess) Run(_ context.Context, request cli.CommandRequest) error {
	runner.requests = append(runner.requests, request)
	return nil
}

func TestComputerWorkspaceUsesInjectedBrowserRunner(t *testing.T) {
	previous := computerWorkspaceProcess
	runner := &fakeWorkspaceProcess{}
	computerWorkspaceProcess = runner
	t.Cleanup(func() { computerWorkspaceProcess = previous })
	if err := openComputerWorkspace(context.Background(), "run-0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if len(runner.requests) != 1 || runner.requests[0].Capability != "open-computer-workspace" || !reflect.DeepEqual(runner.requests[0].Args, []string{"offgrid://computer?task=run-0123456789abcdef0123456789abcdef"}) {
		t.Fatalf("browser requests = %#v", runner.requests)
	}
	if err := openComputerWorkspace(context.Background(), "invalid-task"); err == nil || len(runner.requests) != 1 {
		t.Fatalf("invalid task reached browser runner: error=%v requests=%#v", err, runner.requests)
	}
}

func TestLlamaServiceDiscoveryUsesInjectedProcessRunner(t *testing.T) {
	previous := llamaReloadProcess
	runner := &fakeLlamaReloadRunner{}
	llamaReloadProcess = runner
	t.Cleanup(func() { llamaReloadProcess = previous })
	if !activeLlamaSystemdService() || !reflect.DeepEqual(runner.requests, []string{"llama-systemd-active", "llama-systemd-status"}) {
		t.Fatalf("service lookup requests=%#v", runner.requests)
	}
}

func TestReviewedComputerWorkspaceLaunch(t *testing.T) {
	for _, link := range []string{
		"offgrid://computer",
		"offgrid://computer?task=run-0123456789abcdef0123456789abcdef",
	} {
		program, args, err := reviewedProcessSpec(cli.CommandRequest{Capability: "open-computer-workspace", Args: []string{link}})
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "windows" && (program != "rundll32.exe" || !reflect.DeepEqual(args, []string{"url.dll,FileProtocolHandler", link})) {
			t.Fatalf("Windows launch %s %#v", program, args)
		}
	}
	for _, link := range []string{"https://example.com", "offgrid://computer?task=run-", "offgrid://computer?task=run-0123456789abcdef0123456789abcdeZ"} {
		if _, _, err := reviewedProcessSpec(cli.CommandRequest{Capability: "open-computer-workspace", Args: []string{link}}); err == nil {
			t.Fatalf("accepted unreviewed link %q", link)
		}
	}
}
