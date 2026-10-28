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

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestInteractiveStreamFailurePreservesStoppedTurnWithoutResubmission(t *testing.T) {
	for _, status := range []string{"failed", "interrupted", "uncertain", "completed", "running", "foreign-turn"} {
		t.Run(status, func(t *testing.T) {
			requestID := ""
			submitted, cancelled := 0, 0
			client, err := serviceclient.New("http://fixture", "", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				var body any
				switch {
				case r.URL.Path == "/api/v2/system":
					body = map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
				case r.URL.Path == "/v1/sessions/conversation":
					body = map[string]any{"name": "conversation", "created_at": "2026-10-26T00:00:00Z", "updated_at": "2026-10-26T00:00:00Z", "messages": []any{}}
				case r.URL.Path == "/v1/sessions/conversation/turn" || r.URL.Path == "/v1/sessions/conversation/generate":
					if r.Method == http.MethodPost {
						submitted++
						var value map[string]any
						if json.NewDecoder(r.Body).Decode(&value) != nil {
							t.Fatal("invalid submission")
						}
						requestID = value["request_id"].(string)
					}
					if requestID == "" {
						body = map[string]any{"turn": nil}
						break
					}
					id := requestID
					state := status
					if r.Method == http.MethodPost {
						state = "running"
					} else if status == "foreign-turn" {
						id = "different-request"
						state = "failed"
					}
					body = map[string]any{"turn": map[string]any{"id": id, "job_id": "run-root", "status": state}, "job": map[string]any{"run_id": "run-root", "status": state, "output": "verified answer"}}
				case r.URL.Path == "/api/v2/jobs/run-root/events":
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("fixture"))}, nil
				case r.URL.Path == "/api/v2/jobs/run-root/cancel":
					cancelled++
					body = map[string]any{"run_id": "run-root", "status": "cancelled"}
				default:
					t.Fatal("unexpected request", r.Method, r.URL)
				}
				data, _ := json.Marshal(body)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = RunConversationChat(context.Background(), client, []string{"chat", "conversation", "--profile", "profile-fixture", "--profile-revision", "1"}, strings.NewReader("Do work\n/exit\n"), &out, false, func(io.Writer, io.Writer, io.Reader) error { return io.ErrUnexpectedEOF })
			known := status != "running" && status != "foreign-turn"
			if known && err != nil || !known && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatal(status, err, out.String())
			}
			if submitted != 1 || cancelled != map[bool]int{true: 1, false: 0}[status == "running"] {
				t.Fatal("resubmitted or stopped wrong root", submitted, cancelled)
			}
			if known && strings.Count(out.String(), "\n> ") != 2 {
				t.Fatal("terminal turn closed conversation", out.String())
			}
			if status == "completed" && !strings.Contains(out.String(), "verified answer") {
				t.Fatal("completed saved answer was lost")
			}
		})
	}
}
