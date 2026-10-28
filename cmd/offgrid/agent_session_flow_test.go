package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type sessionFixtureTransport func(*http.Request) (*http.Response, error)

func (f sessionFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentSessionPTYConversation(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_SESSION_PTY") != "1" {
		t.Skip("isolated PTY fixture only")
	}
	submitted := 0
	clientHTTP := &http.Client{Transport: sessionFixtureTransport(func(r *http.Request) (*http.Response, error) {
		var body any
		switch r.URL.Path {
		case "/api/v2/system":
			body = map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
		case "/api/v2/agent-profiles/profile-fixture":
			body = map[string]any{"id": "profile-fixture", "revision": 7, "settings": map[string]any{"name": "bumblebee", "model": "fixture-model", "max_iterations": 30}}
		case "/v1/sessions/fixture":
			body = map[string]any{"name": "fixture", "created_at": "2026-10-26T00:00:00Z", "updated_at": "2026-10-26T00:00:00Z"}
		case "/v1/sessions/fixture/turn":
			body = map[string]any{"turn": nil}
		case "/v1/sessions/fixture/generate":
			submitted++
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request["content"] != "hello" {
				t.Fatal("unexpected submission", request, err)
			}
			body = map[string]any{"turn": map[string]any{"id": request["request_id"], "job_id": "run-fixture", "status": "running"}, "job": map[string]any{"run_id": "run-fixture", "status": "running"}}
		case "/api/v2/jobs/run-fixture/events":
			reader, writer := io.Pipe()
			go func() {
				defer writer.Close()
				fmt.Fprintln(writer, `data: {"type":"status","run_id":"run-fixture","status":"running","progress":{"phase":"loading","iteration":1}}`)
				time.Sleep(500 * time.Millisecond)
				fmt.Fprintln(writer, `data: {"type":"status","run_id":"run-fixture","status":"running","progress":{"phase":"generating","iteration":1,"preview":"Hello from the fixture"}}`)
				time.Sleep(500 * time.Millisecond)
				fmt.Fprintln(writer, `data: {"type":"done","run_id":"run-fixture","status":"completed","output":"Hello from the fixture!"}`)
			}()
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}, nil
		default:
			t.Fatal("unexpected service operation", r.Method, r.URL)
		}
		data, _ := json.Marshal(body)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}
	client, err := serviceclient.New("http://isolated-fixture:11611", "fixture", clientHTTP)
	if err != nil {
		t.Fatal(err)
	}
	env := cli.Environment{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv, Setenv: os.Setenv, HTTP: clientHTTP, LongHTTP: clientHTTP, Exec: reviewedProcessRunner{},
		Service: cli.ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }, RenderStream: renderAgentStreamTo, ReadAgentLine: newAgentSessionInput(), Chat: func(ctx context.Context, c *serviceclient.Client, args []string, in io.Reader, out io.Writer, j bool) error {
			return cli.RunConversationChat(ctx, c, args, in, out, j, renderAgentStreamTo)
		}},
	}
	if code := cli.Execute(context.Background(), []string{"agent", "chat", "--profile", "profile-fixture", "--resume", "fixture"}, env); code != 0 {
		t.Fatal("conversation failed", code)
	}
	if submitted != 1 {
		t.Fatal("duplicated or missing turn", submitted)
	}
}
