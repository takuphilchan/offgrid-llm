package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestTaskSubmissionUsesDurableAPIAndRequestIdentity(t *testing.T) {
	req, wait, err := parseTaskSubmission([]string{"Compare sources", "--model", "qwen", "--request-id", "same-request", "--wait", "--max-steps", "20"})
	if err != nil || !wait || req.MaxIterations != 20 {
		t.Fatal(req, wait, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v2/jobs" || r.Header.Get("Authorization") != "Bearer key" {
			t.Error("wrong transport")
		}
		var got taskSubmission
		if json.NewDecoder(r.Body).Decode(&got) != nil || got != req {
			t.Error("request changed")
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"run_id":"run-saved","status":"waiting_for_children"}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "key", server.Client())
	if result, err := submitTask(context.Background(), client, req); err != nil || result.RunID != "run-saved" {
		t.Fatal(result, err)
	}
	for _, args := range [][]string{{"--model", "qwen"}, {"work", "--model", "qwen", "--max-steps", "0"}, {"work", "--model", "qwen", "--unknown", "yes"}} {
		if _, _, err := parseTaskSubmission(args); err == nil {
			t.Fatal("bad options accepted", args)
		}
	}
}

func TestTaskEvidenceExportPreservesStructuredSteps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v2/jobs/run-123/export" {
			t.Error("wrong route")
		}
		w.Write([]byte(`{"run_id":"run-123","status":"completed","steps":[{"tool_name":"computer_verify","tool_result":"evidence"}],"format":"offgrid-task-evidence-v1"}`))
	}))
	defer server.Close()
	result, err := agentControl(server.URL, []string{"export", "run-123"})
	if err != nil || !strings.Contains(string(result.Evidence), "computer_verify") {
		t.Fatal("export discarded evidence", err)
	}
}
