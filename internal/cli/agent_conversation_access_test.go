package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestInteractiveOnDemandAccessPreservesBufferedFollowupsAndOneLease(t *testing.T) {
	var out, diagnostics bytes.Buffer
	env := testEnvironment(&out, &diagnostics, &fakeRunner{})
	lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/fixture", Directory: ".", Backend: "local", Permissions: execution.Permissions{Read: true}}}
	opened, attached, turn := 0, 0, 0
	state, requestID, runID := "completed", "", ""
	prompts := []string{}
	env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, folder string, p execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
		opened++
		if folder != "/fixture" || !approve(execution.LocalApproval{Root: folder, Host: "host", Service: "http://fixture", Permissions: p}) {
			t.Fatal("missing exact approval")
		}
		return lease, nil
	}
	client, err := serviceclient.New("http://fixture", "", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		var body any
		switch {
		case r.URL.Path == "/api/v2/system":
			body = map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
		case r.URL.Path == "/v1/sessions/conversation":
			body = map[string]any{"name": "conversation", "messages": []any{}, "created_at": "2026-10-26T00:00:00Z", "updated_at": "2026-10-26T00:00:00Z"}
		case strings.HasSuffix(r.URL.Path, "/generate") || strings.HasSuffix(r.URL.Path, "/turn"):
			if r.Method == http.MethodPost {
				var payload map[string]any
				if json.NewDecoder(r.Body).Decode(&payload) != nil {
					t.Fatal("invalid payload")
				}
				turn++
				prompts = append(prompts, payload["content"].(string))
				requestID = payload["request_id"].(string)
				runID = fmt.Sprintf("run-%d", turn)
				state = "running"
				if payload["execution_request_version"] != float64(1) {
					t.Fatal("access contract omitted")
				}
				if turn <= 2 && payload["execution"] != nil {
					t.Fatal("authority before approval")
				}
				if turn > 2 && payload["execution"] == nil {
					t.Fatal("approved follow-up lost workspace")
				}
			}
			body = map[string]any{"turn": nil}
			if requestID != "" {
				body = map[string]any{"turn": map[string]any{"id": requestID, "job_id": runID, "status": state}, "job": map[string]any{"run_id": runID, "status": state}}
			}
		case strings.HasSuffix(r.URL.Path, "/events"):
		case strings.HasSuffix(r.URL.Path, "/input"):
			attached++
			state = "running"
			body = map[string]any{"run_id": runID, "status": state}
		case r.URL.Path == "/api/v2/jobs/"+runID:
			body = map[string]any{"run_id": runID, "status": state}
			if state == "waiting_for_input" {
				body.(map[string]any)["pending_input"] = map[string]any{"id": "input-1", "kind": "workspace", "mode": "read"}
			}
		default:
			t.Fatal("unexpected request", r.URL.Path)
		}
		data, _ := json.Marshal(body)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	env.Service.RenderStream = func(io.Writer, io.Writer, io.Reader) error {
		state = "completed"
		if turn == 2 && attached == 0 {
			state = "waiting_for_input"
		}
		return nil
	}
	workspace := &conversationWorkspace{request: TaskSubmission{Folder: "/fixture", ExecutionRequestVersion: 1}, env: env}
	err = runConversationChatWithAccess(context.Background(), client, []string{"chat", "conversation", "--profile", "profile-fixture", "--profile-revision", "1"}, strings.NewReader("hello\nlist files\nyes\ntell me a story\nread fixture.txt\n/exit\n"), &out, false, env.Service.RenderStream, workspace)
	if err != nil {
		t.Fatal(err, out.String())
	}
	if strings.Join(prompts, "|") != "hello|list files|tell me a story|read fixture.txt" || opened != 1 || attached != 1 || workspace.lease != lease || lease.closed != 0 {
		t.Fatal("buffer or lease lost", prompts, opened, attached, lease.closed)
	}
}
