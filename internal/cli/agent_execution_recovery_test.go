package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestExecutionRecoveryCLIOnlyRequestsOriginalReceipt(t *testing.T) {
	calls := 0
	httpClient := &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v2/jobs/run-123/recover-execution" || r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Fatal(r.Method, r.URL.Path)
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 0 {
			t.Fatal("recovery included replacement work or execution flags", body)
		}
		return &http.Response{StatusCode: 202, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"run_id":"run-123","status":"interrupted"}`))}, nil
	})}
	client, err := serviceclient.New("http://fixture:11611", "fixture-secret", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = AgentControlService(context.Background(), client, []string{"recover-execution", "run-123"}); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	if _, err = AgentControlService(context.Background(), client, []string{"recover-execution", "run-123", "replacement"}); err == nil || calls != 1 {
		t.Fatal("extra recovery input accepted", err)
	}
	var output bytes.Buffer
	RenderAgentSnapshot(&output, AgentRunSnapshot{RunID: "run-123", Status: "uncertain", CanRecoverExecution: true})
	if !strings.Contains(output.String(), "offgrid agent recover-execution run-123") {
		t.Fatal(output.String())
	}
}
