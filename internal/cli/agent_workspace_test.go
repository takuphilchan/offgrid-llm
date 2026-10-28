package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type fixtureExecutionLease struct {
	resource execution.Resource
	done     chan struct{}
	closed   int
}

func (l *fixtureExecutionLease) Resource() execution.Resource { return l.resource }
func (l *fixtureExecutionLease) Done() <-chan struct{}        { return l.done }
func (l *fixtureExecutionLease) Err() error                   { return nil }
func (l *fixtureExecutionLease) Close() error                 { l.closed++; return nil }

func TestInteractiveFolderApprovalIsLocalOnceAndPreservesBufferedInput(t *testing.T) {
	for _, answer := range []string{"y", "n"} {
		t.Run(answer, func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.Stdin = strings.NewReader(answer + "\n/exit\n")
			env.HTTP = &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				body := `{"product":"offgrid","capabilities":["agent-conversations-v1"]}`
				switch r.URL.Path {
				case "/api/v2/system":
				case "/api/v2/agent-profiles/owned":
					body = `{"id":"owned","revision":1,"settings":{"model":"model","name":"General","max_iterations":30}}`
				case "/v1/sessions":
					var value map[string]string
					json.NewDecoder(r.Body).Decode(&value)
					encoded, _ := json.Marshal(map[string]string{"name": value["name"]})
					body = string(encoded)
				default:
					t.Fatal(r.URL)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture:11611", "", env.HTTP) }
			lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/fixture/selected", Backend: "local", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}, done: make(chan struct{})}
			approvals, chats := 0, 0
			env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, folder string, permissions execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
				if folder != "selected folder" || !permissions.Write || !permissions.Read || permissions.Processes {
					t.Fatal(folder, permissions)
				}
				approvals++
				if !approve(execution.LocalApproval{Root: "/fixture/selected", Host: "host", Service: "http://fixture:11611", Permissions: permissions}) {
					return nil, execution.ErrAuthority
				}
				return lease, nil
			}
			env.Service.Chat = func(_ context.Context, _ *serviceclient.Client, args []string, input io.Reader, _ io.Writer, _ bool) error {
				chats++
				if !strings.Contains(strings.Join(args, " "), "--workspace session") {
					t.Fatal(args)
				}
				data, _ := io.ReadAll(input)
				if string(data) != "/exit\n" {
					t.Fatal("approval consumed chat input", string(data))
				}
				return nil
			}
			code := Execute(context.Background(), []string{"agent", "chat", "--profile", "owned", "--workspace-folder", "selected folder"}, env)
			if answer == "y" {
				if code != 0 || chats != 1 || lease.closed != 1 {
					t.Fatal(code, chats, lease.closed, errOut.String())
				}
			} else if code == 0 || chats != 0 || lease.closed != 0 {
				t.Fatal("decline created a chat/grant", code, chats, lease.closed)
			}
			if approvals != 1 {
				t.Fatal(approvals)
			}
		})
	}
}

func TestWorkspaceStartRequiresInteractiveAuthorityAndStopsOwnedLease(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("yes\n")
	client, _ := serviceclient.New("http://fixture:11611", "", env.HTTP)
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/fixture", Backend: "local", Directory: "."}, done: make(chan struct{})}
	opened := 0
	env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, folder string, permissions execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
		opened++
		if folder != "." || permissions.Write || !permissions.Read {
			t.Fatal(folder, permissions)
		}
		if !approve(execution.LocalApproval{Root: "/fixture", Host: "host", Permissions: permissions}) {
			t.Fatal("fixture approval lost")
		}
		cancel()
		return lease, nil
	}
	if code := Execute(ctx, []string{"agent", "workspace", "start", ".", "--access", "read"}, env); code != 0 || opened != 1 || lease.closed != 1 {
		t.Fatal(code, opened, lease.closed, errOut.String())
	}
	if code := Execute(context.Background(), []string{"--json", "agent", "workspace", "start", "."}, env); code != 2 || opened != 1 {
		t.Fatal("JSON minted authority", code, opened)
	}
}

func TestInteractiveCancellationUnblocksOwnedInput(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	reader, writer := io.Pipe()
	defer writer.Close()
	defer reader.Close()
	env.Stdin = reader
	env.HTTP = &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"product":"offgrid","capabilities":["agent-conversations-v1"]}`
		if r.URL.Path == "/api/v2/agent-profiles/owned" {
			body = `{"id":"owned","revision":1,"settings":{"model":"model","max_iterations":30}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture:11611", "", env.HTTP) }
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.Service.Chat = func(ctx context.Context, _ *serviceclient.Client, _ []string, input io.Reader, _ io.Writer, _ bool) error {
		close(started)
		io.ReadAll(input)
		return ctx.Err()
	}
	done := make(chan int, 1)
	go func() {
		done <- Execute(ctx, []string{"agent", "chat", "--profile", "owned", "--resume", "saved"}, env)
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Ctrl+C remained blocked on stdin")
	}
}
