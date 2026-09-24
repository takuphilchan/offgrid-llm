package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		if r.URL.Path != "/api/v2/jobs/run-123/approve" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
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

func TestAgentStreamShowsLivePreviewOnceOnStderr(t *testing.T) {
	var stream strings.Builder
	for _, text := range []string{"Hel", "Hello", "Hello"} {
		event := map[string]any{"type": "status", "run_id": "run-1", "status": "running", "progress": map[string]any{"phase": "generating", "iteration": 1, "preview": text}}
		data, _ := json.Marshal(event)
		fmt.Fprintf(&stream, "data: %s\n\n", data)
	}
	stream.WriteString("data: {\"type\":\"done\",\"run_id\":\"run-1\",\"status\":\"completed\",\"output\":\"Hello\"}\n\n")
	var out, progress bytes.Buffer
	if err := renderAgentStreamTo(&out, &progress, strings.NewReader(stream.String())); err != nil {
		t.Fatal(err)
	}
	if strings.Count(progress.String(), "Hello") != 1 || !strings.Contains(progress.String(), "Generating response") || !strings.Contains(progress.String(), "not complete") {
		t.Fatalf("wrong progress: %s", progress.String())
	}
	if strings.Contains(out.String(), "preview") || !strings.Contains(out.String(), "Hello") {
		t.Fatalf("wrong final output: %s", out.String())
	}
}

func TestAgentStreamRendersResultAndExactApproval(t *testing.T) {
	for _, tc := range []struct {
		data string
		want []string
	}{
		{`{"type":"done","run_id":"run-1","status":"completed","output":"Finished task"}`, []string{"run-1", "Finished task"}},
		{`{"type":"input_required","run_id":"run-1","status":"waiting_for_input","pending_input":{"id":"input-1","target":"Notepad","kind":"computer","mode":"app"}}`, []string{"Notepad", "saved task", "offgrid agent cancel run-1"}},
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

func TestAgentControlRejectsInvalidSnapshotAndOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"run_id":"wrong-run","status":"completed"}`))
	}))
	defer server.Close()
	for _, base := range []string{server.URL, server.URL + "/wrong", server.URL + "?query=wrong"} {
		if _, err := agentControl(base, []string{"status", "run-1"}); err == nil {
			t.Fatalf("accepted %s", base)
		}
	}
}
