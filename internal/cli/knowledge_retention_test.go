package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestKnowledgeRetentionContract(t *testing.T) {
	const good = `{"version":1,"revision":2,"delete_after":0,"legal_hold":true}`
	for _, tc := range []struct {
		name, response string
		status         int
		valid          bool
	}{
		{"confirmed", good, 200, true},
		{"missing hold", `{"version":1,"revision":2,"delete_after":0}`, 200, false},
		{"missing expiry", `{"version":1,"revision":2,"legal_hold":true}`, 200, false},
		{"stale receipt", `{"version":1,"revision":1,"delete_after":0,"legal_hold":true}`, 200, false},
		{"wrong effect", `{"version":1,"revision":2,"delete_after":0,"legal_hold":false}`, 200, false},
		{"unknown schema", `{"version":2,"revision":2,"delete_after":0,"legal_hold":true}`, 200, false},
		{"conflict", `{"error":"Policy changed"}`, 409, false},
		{"unavailable", `{"error":"Unavailable"}`, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client, err := serviceclient.New("http://fixture", "fixture-key", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "PUT" || r.URL.Path != "/v1/documents/retention" || r.URL.Query().Get("id") != "source&private" || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatal("wrong scope", r.URL)
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 4 || body["revision"] != float64(1) || body["delete_after"] != float64(0) || body["legal_hold"] != true {
					t.Fatal("wrong policy", body)
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.response))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			_, err = RunKnowledgeService(context.Background(), client, []string{"retention", "source&private", "--revision", "1", "--delete-after", "0", "--legal-hold", "true"}, nil, io.Discard, false)
			if (err == nil) != tc.valid || calls != 1 {
				t.Fatalf("valid=%v calls=%d error=%v", tc.valid, calls, err)
			}
		})
	}
	for _, args := range [][]string{
		{"retention"}, {"retention", ""}, {"retention", "one", "--legal-hold", "false"},
		{"retention", "one", "--revision", "1", "--revision", "0", "--legal-hold", "true"},
		{"retention", "one", "--revision", "1", "--delete-after", "-1", "--legal-hold", "true"},
		{"retention", "one", "--revision", "1", "--delete-after", "0", "--legal-hold", "yes"},
	} {
		if _, err := RunKnowledgeService(context.Background(), nil, args, nil, io.Discard, false); err == nil {
			t.Fatalf("accepted incomplete/invalid policy: %v", args)
		}
	}
}
