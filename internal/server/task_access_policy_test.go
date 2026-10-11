package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestTaskAccessPolicySubmission(t *testing.T) {
	for _, mode := range []string{"ask_every_time", "scoped_changes", "full_task", "", "unrestricted"} {
		t.Run(mode, func(t *testing.T) {
			s := taskFirstServer(t)
			body := map[string]string{"prompt": "read documentation", "model": "model", "request_id": "test-policy-001"}
			if mode != "" {
				body["computer_approval_mode"] = mode
			}
			post := func() *httptest.ResponseRecorder {
				encoded, _ := json.Marshal(body)
				w := httptest.NewRecorder()
				s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(string(encoded))))
				return w
			}
			w := post()
			if mode == "unrestricted" {
				if w.Code != 400 {
					t.Fatal(w.Code, w.Body.String())
				}
				return
			}
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var response struct {
				ID     string `json:"run_id"`
				Policy string `json:"requested_computer_approval_mode"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			task, ok := s.agentManager.GetTask(response.ID)
			if !ok || task.Config.RequestedComputerApprovalMode != mode || response.Policy != mode || task.Config.ComputerSession != "" || task.Config.ComputerApprovalMode != "" {
				t.Fatalf("intent became authority or was lost: %+v", task)
			}
			if w = post(); w.Code != 202 || !strings.Contains(w.Body.String(), response.ID) {
				t.Fatal("retry did not reuse task", w.Code, w.Body.String())
			}
			body["computer_approval_mode"] = "ask_every_time"
			if mode == "ask_every_time" {
				body["computer_approval_mode"] = "full_task"
			}
			if w = post(); w.Code != 409 {
				t.Fatal("policy change reused request", w.Code, w.Body.String())
			}
		})
	}
}

func TestTaskAccessPolicyMismatchDoesNotReserveOrDispatch(t *testing.T) {
	s := taskFirstServer(t)
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 8192
	cfg.RequestedComputerApprovalMode = "full_task"
	task, err := s.agentRunner.Create("read documentation", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.PendingInput == nil {
		t.Fatal(err)
	}
	code, _ := s.browserHub.PairCode(task.Actor)
	host, _, err := s.browserHub.Pair(code, "offgrid-demo://research", computer.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"input_id":"` + task.PendingInput.ID + `","computer_session":"` + host.ID + `"}`
	w := httptest.NewRecorder()
	// No engine: entering model preflight at all would be a regression.
	s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", strings.NewReader(body)), task.ID)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "computer_policy_mismatch") {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, _ := s.agentManager.GetTask(task.ID)
	if saved.Status != agents.TaskInput || saved.Config.ComputerSession != "" || saved.Config.RequestedComputerApprovalMode != "full_task" {
		t.Fatal("mismatch changed task")
	}
	if err = s.browserHub.ReserveTask(task.Actor, host.ID, "other-job"); err != nil {
		t.Fatal("mismatch reserved access", err)
	}
}
