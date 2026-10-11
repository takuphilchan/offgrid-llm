package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkflowOmissionKeepsSubmissionDigest(t *testing.T) {
	// This is the original serialized envelope, including its field order.
	original := `{"prompt":"open editor","model":"model","request_id":"old-request","computer_approval_mode":"full_task","computer_scope":"desktop"}`
	var req jobSubmitRequest
	if err := json.Unmarshal([]byte(original), &req); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(req)
	if string(encoded) != original {
		t.Fatalf("changed idempotency identity: %s", encoded)
	}
	s := taskFirstServer(t)
	w := httptest.NewRecorder()
	s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(original)))
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var response struct {
		ID string `json:"run_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	task, _ := s.agentManager.GetTask(response.ID)
	sum := sha256.Sum256([]byte(original))
	if task.RequestDigest != hex.EncodeToString(sum[:]) {
		t.Fatal("legacy hash changed")
	}
	if task.Config.NativeWorkflowVersion != 1 || task.Workflow != nil {
		t.Fatal("evidence should activate only after native access is attached")
	}
}

func TestWorkflowPublicProjectionsAreOwnerScopedAndObservationOnly(t *testing.T) {
	s := taskFirstServer(t)
	cfg := compositionConfig("windows-uia", true)
	cfg.NativeWorkflowVersion = 1
	cfg.NativeExpectations = []agents.OutcomeExpectation{{ID: "research", Title: "Private outcome", Kind: "research"}}
	own, err := s.agentRunner.Create("private fixture", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.agentRunner.Create("foreign fixture", "model", "other", cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "/export"} {
		w := httptest.NewRecorder()
		s.handleJobRead(w, httptest.NewRequest("GET", "/api/v2/jobs/"+own.ID+suffix, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"verification"`) || !strings.Contains(w.Body.String(), `"caller_explicit"`) || strings.Contains(w.Body.String(), `"checkpoint"`) {
			t.Fatal(w.Code, w.Body.String())
		}
		w = httptest.NewRecorder()
		s.handleJobRead(w, httptest.NewRequest("GET", "/api/v2/jobs/"+foreign.ID+suffix, nil))
		if w.Code != 404 || strings.Contains(w.Body.String(), "Private outcome") {
			t.Fatal("foreign evidence disclosed")
		}
	}
	saved, _ := s.agentManager.GetTask(own.ID)
	if saved.Status != own.Status || len(saved.Steps) != 0 {
		t.Fatal("read started work")
	}
	legacy := taskResponse(&agents.Task{Status: agents.TaskCompleted, Result: "Original"})
	if _, exists := legacy["verification"]; exists || legacy["status"] != agents.TaskCompleted {
		t.Fatal("legacy certified")
	}
}
