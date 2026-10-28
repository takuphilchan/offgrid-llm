package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestConversationLeaseSurvivesTurnButNotConversation(t *testing.T) {
	var output bytes.Buffer
	env := testEnvironment(&output, &output, &fakeRunner{})
	env.Stdin = strings.NewReader("yes\n")
	lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/fixture", Permissions: execution.Permissions{Read: true}}}
	var hostContext context.Context
	env.Execution.Open = func(ctx context.Context, _ *serviceclient.Client, folder string, permissions execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
		hostContext = ctx
		if folder != "/fixture" || !approve(execution.LocalApproval{Root: folder, Host: "host", Permissions: permissions}) {
			t.Fatal("wrong authority")
		}
		return lease, nil
	}
	state := "waiting_for_input"
	client, _ := serviceclient.New("http://fixture", "", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && r.URL.Path == "/api/v2/jobs/fixture/input" {
			state = "completed"
		}
		body := `{"run_id":"fixture","status":"` + state + `"}`
		if state == "waiting_for_input" {
			body = `{"run_id":"fixture","status":"waiting_for_input","pending_input":{"id":"input","kind":"workspace","mode":"read"}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var retained ExecutionLease
	if err := runWithWorkspaceInput(ctx, client, TaskSubmission{Folder: "/fixture"}, &AgentRunSnapshot{RunID: "fixture", Status: "waiting_for_input"}, env, &retained); err != nil {
		t.Fatal(err)
	}
	if retained != lease || lease.closed != 0 || hostContext == nil || hostContext.Err() != nil {
		t.Fatal("turn completion revoked retained conversation authority")
	}
	// A subsequent turn observes the same live lease without another grant.
	if err := runWithWorkspaceInput(ctx, client, TaskSubmission{Folder: "/fixture"}, &AgentRunSnapshot{RunID: "fixture", Status: "completed"}, env, &retained); err != nil {
		t.Fatal(err)
	}
	if hostContext.Err() != nil || retained != lease {
		t.Fatal("follow-up lost its host")
	}
	cancel()
	if hostContext.Err() != context.Canceled {
		t.Fatal("conversation cancellation did not reach host")
	}
	if err := closeExecutionLease(retained); err != nil || lease.closed != 1 {
		t.Fatal("conversation did not close authority", err)
	}
}
