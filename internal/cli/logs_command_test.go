package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureLogRunner struct {
	requests []CommandRequest
	active   bool
}

func (runner *fixtureLogRunner) Run(_ context.Context, request CommandRequest) error {
	runner.requests = append(runner.requests, request)
	if request.Capability == "systemd-status" && runner.active {
		_, _ = io.WriteString(request.Stdout, "active\n")
	}
	if request.Capability == "tail-logs" || request.Capability == "journalctl-logs" {
		_, _ = io.WriteString(request.Stdout, "fixture log entry\n")
	}
	return nil
}

func TestLogsUseReviewedRunnerWithoutLaunchingProcesses(t *testing.T) {
	home := t.TempDir()
	logPath := filepath.Join(home, ".offgrid", "logs", "server.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		args           []string
		active         bool
		wantCapability string
		wantArgs       string
	}{
		{"file", []string{"-n", "42"}, false, "tail-logs", fmt.Sprintf("[%s 42 false]", logPath)},
		{"systemd", []string{"-f", "-n", "19"}, true, "journalctl-logs", "[19 true]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			runner := &fixtureLogRunner{active: test.active}
			env := testEnvironment(&out, &errOut, runner)
			env.Logs = LogDependencies{HomeDir: func() (string, error) { return home, nil }, Stat: os.Stat}
			if code := Execute(context.Background(), append([]string{"logs"}, test.args...), env); code != 0 {
				t.Fatalf("code=%d stderr=%q", code, errOut.String())
			}
			if len(runner.requests) != 2 || runner.requests[0].Capability != "systemd-status" || runner.requests[1].Capability != test.wantCapability || fmt.Sprint(runner.requests[1].Args) != test.wantArgs || !strings.Contains(out.String(), "fixture log entry") {
				t.Fatalf("requests=%#v stdout=%q", runner.requests, out.String())
			}
		})
	}
}

func TestLogsRejectUnboundedCountBeforeProcess(t *testing.T) {
	var out, errOut bytes.Buffer
	runner := &fixtureLogRunner{}
	env := testEnvironment(&out, &errOut, runner)
	env.Logs = LogDependencies{HomeDir: func() (string, error) { return t.TempDir(), nil }, Stat: os.Stat}
	if code := Execute(context.Background(), []string{"logs", "-n", "10001"}, env); code != 2 || len(runner.requests) != 0 {
		t.Fatalf("count not rejected: code=%d requests=%d stderr=%q", code, len(runner.requests), errOut.String())
	}
}
