package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestLegacyAgentRoutesThroughInjectedBoundary(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("test prompt\n/exit\n")
	dataDir := t.TempDir()
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ServerPort: 12999, DataDir: dataDir} }
	chats := 0
	env.LegacyAgent = LegacyAgentDependencies{
		SubmitTask: func(_ context.Context, args []string) error {
			chats++
			if !strings.Contains(strings.Join(args, " "), "--model fixture --style react --max-steps 10 --wait -- test prompt") {
				t.Fatalf("arguments=%q", args)
			}
			return nil
		},
		MCP: MCPDependencies{ReadFile: os.ReadFile, MkdirAll: os.MkdirAll, WriteFile: os.WriteFile, Now: time.Now},
	}
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "localhost:12999" {
			t.Fatalf("host=%s", req.URL.Host)
		}
		body := ""
		switch req.URL.Path {
		case "/v1/agents/tasks":
			if req.Header.Get("Authorization") != "Bearer fixture-key" {
				t.Fatal("missing task authentication")
			}
			body = `[{"id":"task-1","status":"completed","prompt":"fixture task"}]`
		case "/v1/agents/workflows":
			body = `[{"name":"fixture workflow","steps":[{}]}]`
		case "/v1/agents/tools":
			body = `{"count":1,"tools":[{"name":"read_file","description":"fixture","type":"builtin"}]}`
		default:
			t.Fatalf("path=%s", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New("http://localhost:12999", "fixture-key", env.HTTP)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"agent", "templates"}, "Agent Templates"},
		{[]string{"agent", "list"}, "fixture task"},
		{[]string{"agent", "workflows"}, "fixture workflow"},
		{[]string{"agent", "tools"}, "Read file contents"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, out.String(), errOut.String())
		}
	}
	for _, args := range [][]string{{"agent", "chat", "--model", "fixture"}, {"agent", "mcp", "list"}, {"agent", "help"}} {
		if code := Execute(context.Background(), args, env); code != 0 {
			t.Fatalf("%v: code=%d", args, code)
		}
	}
	if chats != 1 || !strings.Contains(out.String(), "Agent commands") {
		t.Fatalf("interactive/help result chats=%d stdout=%q", chats, out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"agent", "chat", "--bad"}, env); code != 0 || chats != 1 || !strings.Contains(errOut.String(), "Unknown or incomplete option") {
		t.Fatalf("invalid chat: code=%d chats=%d stderr=%q", code, chats, errOut.String())
	}
}

func TestLegacyAgentInteractiveSelectsModelAndSubmitsOnce(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("write a brief\n/exit\n")
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ServerPort: 12999} }
	requests, submissions := 0, 0
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.String() != "http://localhost:12999/v1/models" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"base"},{"id":"fixture-instruct"}]}`)), Header: http.Header{}}, nil
	})}
	env.LegacyAgent.SubmitTask = func(_ context.Context, args []string) error {
		submissions++
		if strings.Join(args, " ") != "--model fixture-instruct --style react --max-steps 10 --wait -- write a brief" {
			t.Fatalf("wrong task submission: %#v", args)
		}
		return nil
	}
	if code := Execute(context.Background(), []string{"agent", "chat"}, env); code != 0 || requests != 1 || submissions != 1 || !strings.Contains(out.String(), "Session closed.") {
		t.Fatalf("interactive selection: code=%d requests=%d submissions=%d stdout=%q stderr=%q", code, requests, submissions, out.String(), errOut.String())
	}
}
