package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestResponseProfileParsingAndCompatibility(t *testing.T) {
	for _, value := range []string{"interactive", "extended", "invalid"} {
		r, _, err := ParseTaskSubmission([]string{"hello", "--model", "model", "--response-profile", value})
		if value == "invalid" {
			if err == nil {
				t.Fatal("invalid accepted")
			}
			continue
		}
		if err != nil || r.ResponseProfile != value {
			t.Fatal(r, err)
		}
	}
	for _, supported := range []bool{false, true} {
		for _, terminal := range []bool{false, true} {
			for _, detach := range []bool{false, true} {
				calls := 0
				client, _ := serviceclient.New("http://fixture", "", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
					calls++
					body := `{"product":"offgrid","capabilities":[]}`
					if supported {
						body = `{"product":"offgrid","capabilities":["agent-response-profiles-v1"]}`
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
				})})
				env := Environment{}
				env.Execution.Interactive = func() bool { return terminal }
				req := TaskSubmission{Detach: detach, Workspace: "owned-session"}
				if _, err := prepareRunWorkspace(context.Background(), client, &req, env); err != nil {
					t.Fatal(err)
				}
				want := ""
				if supported && terminal && !detach {
					want = "interactive"
				}
				if req.ResponseProfile != want || (!terminal || detach) && calls != 0 {
					t.Fatal(req, calls)
				}
			}
		}
	}
}
