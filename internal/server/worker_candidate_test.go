package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Runs only an owned helper binary in temporary private copies. The helper
// fails unless it sees the editor's sealed bytes instead of the original input.
func TestAutomaticDependentWorkerValidatesSealedEdits(t *testing.T) {
	for _, scenario := range []string{"live", "revoked", "stopped", "integrated", "lost_integration_result", "integrated_followup", "retained_followup"} {
		t.Run(scenario, func(t *testing.T) { testAutomaticDependentWorker(t, scenario) })
	}
}

func testAutomaticDependentWorker(t *testing.T, scenario string) {
	revoked := scenario == "revoked"
	integrating := scenario == "integrated" || scenario == "lost_integration_result" || scenario == "integrated_followup"
	followupIntegration := scenario == "integrated_followup" || scenario == "retained_followup"
	writesDone := make(chan struct{})
	s := profileFixture(t)
	s.config.EnableConversationPreview = integrating || followupIntegration
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	ref, source := approvedExecutionCopiesFixture(t, s, execution.Permissions{Read: true, Write: true, Processes: true}, execution.WorkingCopyPolicy{Version: 1, MaxTotalBytes: 8192, MaxCopyBytes: 1024, MaxEntries: 100, MaxCopies: 4})
	if err := os.WriteFile(filepath.Join(source, "input.txt"), []byte("user's dirty input"), 0600); err != nil {
		t.Fatal(err)
	}
	resource, err := s.executionHosts.Resolve(ctx, "local-admin", s.workspaceID, ref)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"workspace_write", "workspace_process_start", "workspace_process_wait", "workspace_read"}
	broker := &browserRunTools{RunTools: s.toolRegistry, server: s}
	var tools agents.RunTools = broker
	lost := &integrationLostResultFixture{browserRunTools: broker}
	if scenario == "lost_integration_result" {
		tools = lost
	}
	s.agentRunner = agents.NewRunner(s.agentManager, tools, func(callCtx context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		message := api.ChatMessage{Role: "assistant", Content: "Recorded the worker results."}
		call := func(name string, payload any) {
			data, _ := json.Marshal(payload)
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{ID: fmt.Sprintf("call-%d", len(task.Steps)), Type: "function", Function: api.FunctionCall{Name: name, Arguments: string(data)}}}
		}
		if task.ParentID == "" && len(task.ChildHistory) == 0 {
			call("delegate_tasks", map[string]any{"tasks": []agents.ChildSpec{
				{Key: "edit", Mode: "edit", Goal: "Edit the private input", Tools: []string{names[0], names[3]}},
				{Key: "test", Mode: "test", Goal: "Test the exact editing result", Tools: names[1:], DependsOn: []string{"edit"}},
			}})
		} else if task.ParentID == "" && (integrating || followupIntegration && len(task.PreviousWorkerIDs) > 0) {
			for _, entry := range task.Checkpoint.StorageBudget.Reservations {
				if entry.Request.Parent.Job == task.ID && entry.Capture != nil && entry.Capture.Result != nil && entry.Capture.Result.Candidate != nil && entry.Integration == nil {
					call(execution.IntegrateTool, map[string]any{"worker_run_id": entry.Request.WorkerRunID})
					break
				}
			}
			if followupIntegration && len(task.ChildHistory) == 1 && len(message.ToolCalls) == 0 {
				call("delegate_tasks", map[string]any{"tasks": []agents.ChildSpec{{Key: "followup_test", Mode: "test", Goal: "Validate the retained editor's current result", Tools: names[1:], DependsOn: []string{"edit"}}}})
			}
		} else if task.ParentID != "" {
			if !agents.HasWorkerEnvironment(task) || task.Config.ExecutionWorkspace.Root == source {
				t.Error("worker lacks private candidate")
			}
			actions := 0
			var preimage execution.FilePage
			var process execution.ProcessResult
			for _, step := range task.Steps {
				if step.Type != "action" {
					continue
				}
				actions++
				if step.ToolName == names[3] {
					var result struct {
						Receipt execution.Receipt `json:"receipt"`
					}
					if json.Unmarshal([]byte(step.ToolResult), &result) != nil || json.Unmarshal(result.Receipt.Result, &preimage) != nil {
						t.Error("invalid read receipt")
					}
				}
				if task.Config.ExecutionWorkerMode == "test" {
					var result struct {
						Receipt execution.Receipt `json:"receipt"`
					}
					if json.Unmarshal([]byte(step.ToolResult), &result) != nil || json.Unmarshal(result.Receipt.Result, &process) != nil {
						t.Error("invalid test receipt")
					}
				}
			}
			if task.Config.ExecutionWorkerMode == "edit" && actions == 0 {
				call(names[3], map[string]any{"path": "input.txt"})
			} else if task.Config.ExecutionWorkerMode == "edit" && actions == 1 {
				content := "sealed editor result"
				if task.Checkpoint.WorkerContinuation != nil {
					if scenario == "integrated_followup" {
						content = "user edit after integration"
					}
					if task.AssignmentVersion > 2 {
						content = "follow-up retained result"
					}
					if string(preimage.Data) != content {
						t.Errorf("follow-up lost private edits: %q", preimage.Data)
					}
					content = "follow-up retained result"
					if task.AssignmentVersion > 2 {
						content = "continued after stop"
					}
				}
				call(names[0], map[string]any{"path": "input.txt", "content": content, "revision": preimage.Revision})
			} else if scenario == "stopped" && task.Config.ExecutionWorkerMode == "edit" && task.AssignmentVersion == 2 && actions == 2 {
				close(writesDone)
				<-callCtx.Done()
				return nil, callCtx.Err()
			} else if task.Config.ExecutionWorkerMode == "test" {
				if actions == 0 {
					args := []string{"-test.run=^TestWorkerCommandFixture$", "--", "offgrid-worker-edited-fixture"}
					if followupIntegration && len(task.DependencyResults) > 0 {
						parent, _ := s.agentManager.GetTask(task.ParentID)
						if len(parent.PreviousWorkerIDs) > 0 {
							args = append(args, "offgrid-worker-followup-fixture")
						}
					}
					call(names[1], execution.ProcessSpec{Program: os.Args[0], Arguments: args, TimeoutSeconds: 10})
				} else if process.State == "running" {
					call(names[2], map[string]any{"handle": process.Handle, "wait_ms": 1000})
				} else if process.State != "exited" || process.ExitCode == nil || *process.ExitCode != 0 {
					t.Errorf("dependent test failed: %+v", process)
				}
			}
		}
		reason := "stop"
		if len(message.ToolCalls) > 0 {
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: reason, Message: message}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	conversation, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ScopedCapabilities, cfg.MaxIterations, cfg.ContextWindow = true, true, 30, 32768
	cfg.Execution, cfg.ExecutionWorkspace = &ref, &resource
	cfg.AllowedCapabilities = map[string]capabilities.Descriptor{}
	for _, name := range names {
		d, _ := execution.Descriptor(name)
		cfg.AllowedCapabilities[name] = d
	}
	if integrating || followupIntegration {
		d, _ := execution.Descriptor(execution.IntegrateTool)
		cfg.AllowedCapabilities[execution.IntegrateTool] = d
	}
	accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: conversation.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Edit and independently test the result"}}}, "fixture-model", cfg)
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.agentRunner.Continue(ctx, accepted.Work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 2 {
		t.Fatal(root, err)
	}
	s.agentRunner.StartCoordinator(ctx)
	for root.Status != agents.TaskCompleted {
		if scenario == "lost_integration_result" && root.Status == agents.TaskInterrupted {
			if root.ErrorCode != "execution_result_recovered" || lost.calls.Load() != 1 {
				t.Fatal("lost integration was not recovered once", root.ErrorCode, lost.calls.Load())
			}
			break
		}
		if ctx.Err() != nil || root.Status == agents.TaskFailed {
			t.Fatalf("root failed to finish: %+v", root)
		}
		for _, link := range root.ChildHistory {
			child, _ := s.agentManager.GetTask(link.ID)
			if child.Status == agents.TaskInterrupted || child.Status == agents.TaskFailed {
				t.Fatalf("worker failed: %s %s", child.ErrorCode, child.Error)
			}
			// Approval is persisted before the runner releases its active turn.
			// Continue requires that turn to be quiescent, not just a visible card.
			if child.Status == agents.TaskWaiting && child.PendingApproval != nil && !s.agentRunner.WorkerActive(child.ID) {
				if child.Config.ExecutionWorkerMode != "test" || child.PendingApproval.Tool != names[1] {
					t.Fatal("unexpected approval")
				}
				if _, err = s.agentRunner.Continue(ctx, child.ID, scope.Actor, "approve", child.PendingApproval.ID, true); err != nil {
					t.Fatal(err)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
		root, _ = s.agentManager.GetTask(root.ID)
	}
	var editSet string
	for _, reservation := range root.Checkpoint.StorageBudget.Reservations {
		child, _ := s.agentManager.GetTask(reservation.Request.WorkerRunID)
		if reservation.Capture == nil || reservation.Capture.State != "sealed" || reservation.Capture.Result.OperationCount != 1 {
			t.Fatal("incorrect sealed output", reservation)
		}
		if child.Config.ExecutionWorkerMode == "edit" {
			editSet = reservation.Capture.Result.ChangeSetID
		}
	}
	for _, reservation := range root.Checkpoint.StorageBudget.Reservations {
		child, _ := s.agentManager.GetTask(reservation.Request.WorkerRunID)
		if child.Config.ExecutionWorkerMode != "test" {
			continue
		}
		if len(reservation.Request.Prerequisites) != 1 || reservation.Request.Prerequisites[0].ID != editSet {
			t.Fatal("exact dependency not pinned")
		}
		proof := reservation.Capture.Result.Validation
		candidate := reservation.Capture.Result.Candidate
		if candidate == nil || candidate.State != "ready" || candidate.OperationCount != 1 || candidate.ChangeSetID == reservation.Capture.Result.ChangeSetID || candidate.ResultManifestDigest != proof.InputManifestDigest {
			t.Fatal("combined tested edits not retained", candidate)
		}
		changes, err := s.executionHosts.WorkerArtifact(ctx, scope.Actor, scope.Workspace, ref, execution.WorkerArtifactRequest{Operation: "inspect", Source: reservation.Request.Parent, ChangeSetID: candidate.ChangeSetID, Limit: 64})
		if err != nil || changes.Page == nil || len(changes.Page.Operations) != 1 || changes.Page.Operations[0].Path != "input.txt" || changes.Page.ChangeSet.Validation != proof {
			t.Fatal("candidate includes test output or lost validated edit", changes, err)
		}
		if proof.Version != 1 || proof.State != "passed" || proof.CheckCount != 1 || len(proof.InputManifestDigest) != 64 {
			t.Fatal("missing exact candidate validation receipt", proof)
		}
		page, err := s.executionHosts.WorkerArtifact(ctx, scope.Actor, scope.Workspace, ref, execution.WorkerArtifactRequest{Operation: "validation", Source: reservation.Request.Parent, ChangeSetID: reservation.Capture.Result.ChangeSetID, Limit: 1})
		if err != nil || page.Validation == nil || page.Validation.Summary != proof || len(page.Validation.Checks) != 1 || page.Validation.Checks[0].State != "exited" || page.Validation.Checks[0].ExitCode == nil || *page.Validation.Checks[0].ExitCode != 0 {
			t.Fatal("full test evidence not retrievable", page, err)
		}
		data, err := os.ReadFile(filepath.Join(child.Config.ExecutionWorkspace.Root, "validation.txt"))
		if err != nil || string(data) != "input verified" {
			t.Fatal("test did not verify sealed edit", string(data), err)
		}
	}
	if scenario == "retained_followup" {
		testIntegratedWorkerFollowup(t, ctx, s, root, source, scope, false)
		return
	}
	if integrating {
		data, readErr := os.ReadFile(filepath.Join(source, "input.txt"))
		if readErr != nil || string(data) != "sealed editor result" {
			t.Fatal("validated changes not integrated", string(data), readErr)
		}
		if _, statErr := os.Stat(filepath.Join(source, "validation.txt")); !os.IsNotExist(statErr) {
			t.Fatal("test-generated output leaked into original", statErr)
		}
		found := false
		for _, entry := range root.Checkpoint.StorageBudget.Reservations {
			if entry.Integration != nil {
				found = true
				result := agents.WorkerIntegrationOutput(root, entry.Request.WorkerRunID)
				if result == nil || result.State != "integrated" || result.Total != 1 || result.Integrated != 1 {
					t.Fatal("integration evidence missing", result)
				}
			}
		}
		if !found {
			t.Fatal("no service integration reservation")
		}
		testWorkChangeInspection(t, s, root, source)
		// Applying the validated candidate satisfies an explicit edit-access
		// intent without demanding a second direct write to the same file.
		check := *root
		check.Config.ExecutionRequestVersion = execution.AccessRequestVersion
		grant, _ := json.Marshal(map[string]any{"execution": ref, "status": "workspace_attached", "requested_access": "edit", "permissions": execution.Permissions{Read: true, Write: true}})
		check.Steps = append([]agents.Step{{Type: "input_resolved", ToolName: "request_workspace_access", ToolResult: string(grant)}}, root.Steps...)
		if err = (&browserRunTools{server: s}).ValidateExecutionCompletion(&check); err != nil {
			t.Fatal("integrated edit demanded a duplicate write", err)
		}
		check.InstructionVersion++
		if err = (&browserRunTools{server: s}).ValidateExecutionCompletion(&check); err == nil {
			t.Fatal("old integration satisfied corrected work")
		}
		if scenario == "integrated_followup" {
			testIntegratedWorkerFollowup(t, ctx, s, root, source, scope, true)
			return
		}
		s.config.EnableConversationPreview = false
		for _, entry := range root.Checkpoint.StorageBudget.Reservations {
			if entry.Integration == nil {
				continue
			}
			observed, observeErr := broker.ReadWorkerIntegrationReceipt(ctx, root, entry.Integration.Request)
			if observeErr != nil || observed.State != "integrated" {
				t.Fatal("feature disable hid accepted receipts", observed, observeErr)
			}
			if err = s.executionHosts.Revoke(scope.Actor, scope.Workspace, ref); err != nil {
				t.Fatal(err)
			}
			if _, observeErr = broker.ReadWorkerIntegrationReceipt(ctx, root, entry.Integration.Request); observeErr == nil {
				t.Fatal("observation restored revoked source authority")
			}
			testRevokedWorkChangeInspection(t, s, root, entry.Integration.Request.ChangeSetID)
		}
		return
	}
	if root.Checkpoint.StorageBudget.ChargedBytes != 2048 {
		t.Fatal("root storage not shared/settled", root.Checkpoint.StorageBudget.ChargedBytes)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "user's dirty input" {
		t.Fatal("original changed", string(data), err)
	}
	if _, err = os.Stat(filepath.Join(source, "validation.txt")); !os.IsNotExist(err) {
		t.Fatal("test wrote original", err)
	}
	// The same retained worker can continue after the work attempt completed.
	// Its root source must be resolved again, and its child grant/copy are new.
	editor, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
	worker, err := s.agentManager.WorkerSession(scope, editor.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "continue-editor", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Extend the previous private edit"}}
	if revoked {
		if err = s.executionHosts.Revoke(scope.Actor, scope.Workspace, ref); err != nil {
			t.Fatal(err)
		}
	}
	followup, err := s.agentRunner.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if followup.Worker.ID != worker.ID || followup.Worker.RootRunID == root.ID || followup.Worker.CurrentRunID == editor.ID {
		t.Fatal("follow-up identity not retained", followup.Worker)
	}
	stopped := false
	for {
		next, _ := s.agentManager.GetTask(followup.Worker.CurrentRunID)
		parent, _ := s.agentManager.GetTask(followup.Worker.RootRunID)
		if scenario == "stopped" && !stopped {
			select {
			case <-writesDone:
				current, err := s.agentManager.WorkerSession(scope, worker.ID)
				if err != nil {
					t.Fatal(err)
				}
				_, err = s.agentRunner.CommandWorker(scope, api.WorkerCommand{RequestID: "stop-followup", TargetID: worker.ID, ExpectedRevision: current.Revision, Kind: "stop"})
				if err != nil {
					t.Fatal(err)
				}
				stopped = true
			default:
			}
		}
		if revoked && next.Status == agents.TaskInterrupted {
			if next.Config.Execution != nil || next.Checkpoint.WorkerEnvironment != nil || parent.Config.Execution != nil || parent.Checkpoint.StorageBudget.ChargedBytes != 2048 {
				t.Fatal("revoked source was restored or dispatched")
			}
			break
		}
		if ctx.Err() != nil || next.Status == agents.TaskFailed || next.Status == agents.TaskInterrupted && next.ErrorCode != "execution_stopping" {
			t.Fatalf("follow-up did not finish: child=%s %s parent=%s", next.Status, next.Error, parent.Status)
		}
		if parent.Status == agents.TaskCompleted || scenario == "stopped" && parent.Status == agents.TaskFailed {
			wantStatus := agents.TaskCompleted
			if scenario == "stopped" {
				wantStatus = agents.TaskCancelled
			}
			if next.Status != wantStatus || !agents.HasWorkerEnvironment(next) || *next.Config.Execution == *editor.Config.Execution || next.Config.ExecutionWorkspace.Root == editor.Config.ExecutionWorkspace.Root {
				t.Fatal("follow-up reused child execution authority")
			}
			if next.Checkpoint.WorkerContinuation.PreviousRunID != editor.ID || parent.Checkpoint.StorageBudget.ChargedBytes != 3072 {
				t.Fatal("follow-up lost provenance or reopened storage")
			}
			data, err := os.ReadFile(filepath.Join(next.Config.ExecutionWorkspace.Root, "input.txt"))
			if err != nil || string(data) != "follow-up retained result" {
				t.Fatal(string(data), err)
			}
			if scenario == "stopped" {
				captured := false
				for _, reservation := range parent.Checkpoint.StorageBudget.Reservations {
					if reservation.Request.WorkerRunID == next.ID {
						captured = reservation.Capture != nil && reservation.Capture.State == "sealed"
					}
				}
				if !stopped || !captured {
					t.Fatal("partial parent finished before capturing stopped files")
				}
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if retry, err := s.agentRunner.CommandWorker(scope, command); err != nil || retry.Worker.CurrentRunID != followup.Worker.CurrentRunID {
		t.Fatal("duplicate continuation", retry, err)
	}
	if scenario == "stopped" {
		current, err := s.agentManager.WorkerSession(scope, worker.ID)
		if err != nil || !current.Stopped {
			t.Fatal("lost user stop", err)
		}
		continued, err := s.agentRunner.CommandWorker(scope, api.WorkerCommand{RequestID: "explicit-after-stop", TargetID: worker.ID, ExpectedRevision: current.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Continue the retained stopped edit"}})
		if err != nil {
			t.Fatal(err)
		}
		for {
			next, _ := s.agentManager.GetTask(continued.Worker.CurrentRunID)
			parent, _ := s.agentManager.GetTask(continued.Worker.RootRunID)
			if ctx.Err() != nil || next.Status == agents.TaskFailed || next.Status == agents.TaskInterrupted {
				t.Fatalf("stopped continuation failed: %s %s", next.Status, next.Error)
			}
			if parent.Status == agents.TaskCompleted {
				data, err := os.ReadFile(filepath.Join(next.Config.ExecutionWorkspace.Root, "input.txt"))
				if err != nil || string(data) != "continued after stop" {
					t.Fatal(string(data), err)
				}
				if parent.Checkpoint.StorageBudget.ChargedBytes != 4096 {
					t.Fatal("stopped continuation reopened storage")
				}
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	data, err = os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "user's dirty input" {
		t.Fatal("follow-up edited original", string(data), err)
	}
}
