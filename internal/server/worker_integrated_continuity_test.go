package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func testIntegratedWorkerFollowup(t *testing.T, ctx context.Context, s *Server, root *agents.Task, source string, scope agents.ControlScope, alreadyIntegrated bool) {
	t.Helper()
	editor, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
	worker, err := s.agentManager.WorkerSession(scope, editor.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if alreadyIntegrated {
		if err = os.WriteFile(filepath.Join(source, "input.txt"), []byte("user edit after integration"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := api.WorkerCommand{RequestID: "followup-integrated", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Extend the current file and independently validate it"}}
	continued, err := s.agentRunner.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if continued.Worker.ID != worker.ID || continued.Worker.RootRunID == root.ID {
		t.Fatal("worker session not retained")
	}
	if response := workChangeHTTP(s, "/api/v2/work-items/"+root.WorkID+"/changes"); response.Code != 200 {
		t.Fatal("new attempt hid retained change identities", response.Code, response.Body.String())
	}
	for {
		parent, _ := s.agentManager.GetTask(continued.Worker.RootRunID)
		if ctx.Err() != nil || parent.Status == agents.TaskFailed || parent.Status == agents.TaskInterrupted || parent.Status == agents.TaskUncertain {
			t.Fatalf("follow-up root did not complete: %s %s", parent.Status, parent.Error)
		}
		for _, link := range parent.ChildHistory {
			child, _ := s.agentManager.GetTask(link.ID)
			if child.Status == agents.TaskFailed || child.Status == agents.TaskInterrupted {
				t.Fatalf("follow-up worker failed: %s %s", child.Status, child.Error)
			}
			if child.Status == agents.TaskWaiting && child.PendingApproval != nil {
				if child.PendingApproval.Tool != "workspace_process_start" {
					t.Fatal("unexpected approval", child.PendingApproval.Tool)
				}
				if _, err = s.agentRunner.Continue(ctx, child.ID, scope.Actor, "approve", child.PendingApproval.ID, true); err != nil {
					t.Fatal(err)
				}
			}
		}
		if parent.Status == agents.TaskCompleted {
			if len(parent.ChildHistory) != 2 {
				t.Fatal("completed editor duplicated", parent.ChildHistory)
			}
			followup, _ := s.agentManager.GetTask(continued.Worker.CurrentRunID)
			if followup.Worker.SessionID != editor.Worker.SessionID || followup.Checkpoint.WorkerContinuation.PreviousRunID != editor.ID || followup.Config.ExecutionWorkspace.Root == editor.Config.ExecutionWorkspace.Root {
				t.Fatal("follow-up identity/environment incorrect")
			}
			integrations, inherited := 0, false
			for _, entry := range parent.Checkpoint.StorageBudget.Reservations {
				if entry.Request.WorkerRunID == followup.ID {
					inherited = len(entry.Request.Integrated) == 1
				}
				if entry.Integration != nil && entry.Integration.State == "integrated" {
					integrations++
				}
			}
			wantIntegrations := 1
			if alreadyIntegrated {
				wantIntegrations++
			}
			if inherited != alreadyIntegrated || integrations != wantIntegrations || parent.Checkpoint.StorageBudget.ChargedBytes <= root.Checkpoint.StorageBudget.ChargedBytes || parent.Checkpoint.BudgetedModelCalls <= root.Checkpoint.BudgetedModelCalls {
				t.Fatal("follow-up lost receipts or reopened budgets", integrations, inherited)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "follow-up retained result" {
		t.Fatal("follow-up was not integrated", string(data), err)
	}
	if _, err = os.Stat(filepath.Join(source, "validation.txt")); !os.IsNotExist(err) {
		t.Fatal("test output integrated", err)
	}
	if again, err := s.agentRunner.CommandWorker(scope, command); err != nil || again.Worker.CurrentRunID != continued.Worker.CurrentRunID {
		t.Fatal("continuation retry created work", err)
	}
}
