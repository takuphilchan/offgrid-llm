package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestTerminalOneShotOwnsOneWorkspaceAndStreamsCanonicalResult(t *testing.T) {
	for _, mode := range []string{"complete", "declined", "unavailable", "broken-stream", "cancelled", "detach", "json", "pipe", "explicit-folder"} {
		t.Run(mode, func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.Stdin = strings.NewReader("yes\n")
			if mode == "declined" {
				env.Stdin = strings.NewReader("no\n")
			}
			terminal := mode != "pipe" && mode != "explicit-folder"
			env.Execution.Interactive = func() bool { return terminal }
			env.Options.JSON = mode == "json"
			state := "running"
			submitted, streamed, stopped, opened := 0, 0, 0, 0
			lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/selected/local", Directory: ".", Backend: "local", Permissions: execution.Permissions{Read: true, Write: true}}, done: make(chan struct{})}
			env.Execution.Available = func(context.Context, *serviceclient.Client) (bool, error) { return mode != "unavailable", nil }
			env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, folder string, permissions execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
				opened++
				if permissions.Processes || !permissions.Read || !permissions.Write {
					t.Fatal("scope", permissions)
				}
				if mode == "explicit-folder" && folder != "." {
					t.Fatal(folder)
				}
				if !approve(execution.LocalApproval{Root: "/selected/local", Host: "host", Service: "http://fixture", Permissions: permissions}) {
					return nil, execution.ErrAuthority
				}
				return lease, nil
			}
			env.HTTP = &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				body := `{"run_id":"fixture","status":"` + state + `"}`
				switch {
				case r.Method == "GET" && r.URL.Path == "/api/v2/system":
					body = `{"product":"offgrid","capabilities":[]}`
				case r.Method == "POST" && r.URL.Path == "/api/v2/jobs":
					submitted++
					var value map[string]any
					if json.NewDecoder(r.Body).Decode(&value) != nil {
						t.Fatal("payload")
					}
					_, attached := value["execution"]
					want := mode == "complete" || mode == "broken-stream" || mode == "cancelled" || mode == "explicit-folder"
					if attached != want {
						t.Fatal("incorrect workspace binding", value)
					}
					for _, field := range []string{"Folder", "folder", "access", "detach", "Workspace"} {
						if _, ok := value[field]; ok {
							t.Fatal("client-local metadata leaked", field)
						}
					}
				case r.URL.Path == "/api/v2/jobs/fixture/events":
					body = "fixture-stream"
				case r.Method == "GET" && r.URL.Path == "/api/v2/jobs/fixture":
				case r.Method == "POST" && r.URL.Path == "/api/v2/jobs/fixture/cancel":
					stopped++
					state = "cancelled"
					body = `{"run_id":"fixture","status":"cancelled"}`
				default:
					t.Fatal("unexpected request", r.Method, r.URL)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture", "", env.HTTP) }
			env.Service.RenderStream = func(w io.Writer, _ io.Writer, _ io.Reader) error {
				streamed++
				if mode == "broken-stream" {
					return io.ErrUnexpectedEOF
				}
				if mode == "cancelled" {
					return context.Canceled
				}
				state = "completed"
				_, err := io.WriteString(w, "exact local listing\n")
				return err
			}
			args := []string{"run", "List files in the selected folder", "--model", "model"}
			if mode == "detach" {
				args = append(args, "--detach")
			}
			if mode == "explicit-folder" {
				args = append(args, "--workspace-folder", ".")
			}
			err := RunAgentCommand(context.Background(), args, env)
			switch mode {
			case "complete", "explicit-folder":
				if err != nil || submitted != 1 || streamed != 1 || stopped != 0 || opened != 1 || lease.closed != 1 || !strings.Contains(out.String(), "exact local listing") {
					t.Fatal(err, submitted, streamed, stopped, opened, lease.closed, out.String())
				}
			case "declined", "unavailable":
				if err == nil || submitted != 0 || streamed != 0 || lease.closed != 0 {
					t.Fatal("fallback after no grant", err, submitted, streamed)
				}
			case "broken-stream", "cancelled":
				if err == nil || submitted != 1 || streamed != 1 || stopped != 1 || lease.closed != 1 {
					t.Fatal(err, submitted, streamed, stopped, lease.closed)
				}
				if mode == "cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			default:
				if err != nil || submitted != 1 || streamed != 0 || opened != 0 {
					t.Fatal("async compatibility", err, submitted, streamed, opened)
				}
			}
		})
	}
}

func TestOneShotDetachAndFolderConflictsRejectedBeforeEffects(t *testing.T) {
	for _, tail := range [][]string{{"--detach", "--wait"}, {"--detach", "--workspace-folder", "."}, {"--workspace-folder", ".", "--workspace", "existing"}, {"--workspace-access", "arbitrary"}} {
		args := append([]string{"task", "--model", "model"}, tail...)
		if _, _, err := ParseTaskSubmission(args); err == nil {
			t.Fatal(tail)
		}
	}
}
