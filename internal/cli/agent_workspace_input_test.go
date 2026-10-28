package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestConversationFirstOneShotRequestsWorkspaceOnlyWhenNeeded(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(map[bool]string{false: "conversation", true: "local-work"}[local], func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.Stdin = strings.NewReader("yes\n")
			env.Execution.Interactive = func() bool { return true }
			env.Execution.OnDemand = func(context.Context, *serviceclient.Client) (bool, error) { return true, nil }
			env.Execution.CurrentFolder = func() (string, error) { return "/captured-launch", nil }
			opened, submitted, attached, streams := 0, 0, 0, 0
			lease := &fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}, Root: "/captured-launch", Directory: ".", Backend: "local", Permissions: execution.Permissions{Read: true}}}
			env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, folder string, p execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
				opened++
				if submitted != 1 || folder != "/captured-launch" || p.Write || p.Processes || !p.Read {
					t.Fatal("eager or excessive scope", submitted, folder, p)
				}
				if !approve(execution.LocalApproval{Root: folder, Host: "host", Service: "http://fixture", Permissions: p}) {
					t.Fatal("approval missing")
				}
				return lease, nil
			}
			state := "running"
			env.HTTP = &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				switch r.Method + " " + r.URL.Path {
				case "POST /api/v2/jobs":
					submitted++
					var req TaskSubmission
					if json.NewDecoder(r.Body).Decode(&req) != nil || req.Execution != nil || req.ExecutionRequestVersion != 1 {
						t.Fatal("unexpected admission")
					}
				case "GET /api/v2/jobs/fixture/events":
				case "GET /api/v2/jobs/fixture":
				case "POST /api/v2/jobs/fixture/input":
					attached++
					var req struct {
						Input string              `json:"input_id"`
						Ref   execution.Reference `json:"execution"`
					}
					if json.NewDecoder(r.Body).Decode(&req) != nil || req.Input != "input-work" || req.Ref != lease.resource.Reference {
						t.Fatal("attachment changed identity")
					}
					state = "running"
				case "GET /api/v2/system":
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"product":"offgrid","capabilities":[]}`))}, nil
				default:
					t.Fatal("unexpected request", r.Method, r.URL)
				}
				body := `{"run_id":"fixture","status":"` + state + `","output":"Done"}`
				if state == "waiting_for_input" {
					body = `{"run_id":"fixture","status":"waiting_for_input","pending_input":{"id":"input-work","kind":"workspace","mode":"read"}}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture", "", env.HTTP) }
			env.Service.RenderStream = func(w io.Writer, _ io.Writer, _ io.Reader) error {
				streams++
				if local && streams == 1 {
					state = "waiting_for_input"
				} else {
					state = "completed"
					io.WriteString(w, "Done\n")
				}
				return nil
			}
			if err := RunAgentCommand(context.Background(), []string{"run", "test request", "--model", "model"}, env); err != nil {
				t.Fatal(err)
			}
			if submitted != 1 || strings.Count(out.String(), "Done") != 1 {
				t.Fatal("duplicated job/answer", submitted, out.String())
			}
			if local {
				if opened != 1 || attached != 1 || streams != 2 || lease.closed != 1 {
					t.Fatal(opened, attached, streams, lease.closed)
				}
			} else if opened != 0 || attached != 0 || strings.Contains(out.String(), "Allow OffGrid") {
				t.Fatal("conversation asked for files", out.String())
			}
		})
	}
}
