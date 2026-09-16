package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentControlUsesExistingRunAndAuthenticatedRequest(t *testing.T) {
	t.Setenv("OFFGRID_API_KEY", "test-key")
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/agents/tasks/run-123/approve" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var data map[string]any
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			t.Error(err)
		}
		if data["approval_id"] != "approval-123" || data["async"] != true || len(data) != 2 {
			t.Errorf("unexpected action body: %#v", data)
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"run_id":"run-123","status":"running"}`))
	}))
	defer server.Close()
	result, err := agentControl(server.URL, []string{"approve", "run-123", "approval-123"})
	if err != nil || result.RunID != "run-123" {
		t.Fatalf("result: %+v %v", result, err)
	}
	for _, args := range [][]string{{"approve", "run-123"}, {"cancel", "../escape"}, {"resume", "run-123", "prompt"}, {"reconcile", "run-123", "call-1", ""}} {
		if _, err := agentControl(server.URL, args); err == nil {
			t.Errorf("accepted malformed command: %v", args)
		}
	}
	if calls != 1 {
		t.Fatalf("invalid commands reached service: %d", calls)
	}
}

func TestAgentStreamRendersResultAndExactApproval(t *testing.T) {
	for _, tc := range []struct {
		data string
		want []string
	}{
		{`{"type":"done","run_id":"run-1","status":"completed","output":"Finished task"}`, []string{"run-1", "Finished task"}},
		{`{"type":"approval_required","run_id":"run-1","status":"waiting_for_approval","pending_approval":{"id":"approval-1","tool":"write_file","arguments":{"path":"notes.txt"}}}`, []string{"notes.txt", "offgrid agent approve run-1 approval-1", "offgrid agent deny run-1 approval-1"}},
	} {
		var buffer bytes.Buffer
		if err := renderAgentStream(&buffer, strings.NewReader("data: "+tc.data+"\n\n")); err != nil {
			t.Fatal(err)
		}
		for _, want := range tc.want {
			if !strings.Contains(buffer.String(), want) {
				t.Errorf("missing %q in %s", want, buffer.String())
			}
		}
	}
	if err := renderAgentStream(&bytes.Buffer{}, strings.NewReader("data: {\"type\":\"status\",\"run_id\":\"run-1\",\"status\":\"running\"}\n\n")); err == nil {
		t.Fatal("truncated stream appeared successful")
	}
	if safe := terminalSafe("\x1b[31mtest\u202e\r"); strings.ContainsAny(safe, "\x1b\u202e\r") {
		t.Fatalf("unsafe output: %q", safe)
	}
}
