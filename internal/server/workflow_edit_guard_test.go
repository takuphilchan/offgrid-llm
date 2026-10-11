package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkflowEditRequiresExactBoundCheck(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		for _, kind := range []string{"control_text_equals", "control_checked_equals"} {
			for _, fault := range []string{"none", "missing", "application-only", "wrong-value", "wrong-kind", "wrong-control", "wrong-session", "wrong-target", "replacement", "ambiguous", "new-reference", "legacy-host", "legacy-host-new-observation", "stale", "revoked", "unknown-version", "legacy-task", "unsupported-control"} {
				t.Run(driver+"/"+kind+"/"+fault, func(t *testing.T) {
					target := computer.TargetIdentity{ID: "window", Driver: driver, OSSession: "os", ProcessGeneration: "process", Surface: "surface"}
					text, checked := "OffGrid test — café 日本語 12345", false
					expected := agents.OutcomeExpectation{ID: "edit", Title: "Requested edit", Kind: kind, TargetID: target.ID, Element: "control"}
					args := map[string]any{"observation_id": "view", "element": "control"}
					tool := "computer_replace_text"
					if kind == "control_text_equals" {
						expected.Text, args["text"] = &text, text
					} else {
						tool, expected.Checked, args["checked"] = "computer_set_checked", &checked, checked
					}
					view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "control", Identity: "stable", Writable: true, Checkable: true}}}
					marshal := func(v any) string { data, _ := json.Marshal(v); return string(data) }
					task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, NativeWorkflowVersion: 1, ComputerDriver: driver, ComputerSession: "session"}, Workflow: &agents.WorkflowEvidence{Version: 1, Checks: []agents.OutcomeCheck{{Expectation: expected}}}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: marshal(view)}}}
					tools := &browserRunTools{}
					if err := tools.BindWorkflowExpectations(task); err != nil {
						t.Fatal(err)
					}
					check := &task.Workflow.Checks[0]
					switch fault {
					case "missing", "legacy-task":
						task.Workflow.Checks = nil
					case "application-only":
						check.Expectation = agents.OutcomeExpectation{ID: "app", Kind: "application_open", ApplicationID: "app"}
					case "wrong-value":
						if kind == "control_text_equals" {
							args["text"] = "Other"
						} else {
							args["checked"] = true
						}
					case "wrong-kind":
						check.Expectation.Kind = "saved_file"
					case "wrong-control":
						check.Binding.Identity = "other"
					case "wrong-session":
						check.Binding.Session = "other"
					case "wrong-target":
						check.Binding.TargetSHA256 = "other"
					case "replacement":
						view.Observation.Target.ProcessGeneration = "replacement"
					case "ambiguous":
						view.Elements = append(view.Elements, computer.NativeElement{ID: "other", Identity: "stable"})
					case "new-reference":
						view.Observation.ID, view.Elements[0].ID, args["observation_id"], args["element"] = "new-view", "new-control", "new-view", "new-control"
					case "legacy-host", "legacy-host-new-observation":
						check.Binding.Identity, view.Elements[0].Identity = "", ""
					case "stale":
						args["observation_id"] = "old"
					case "revoked":
						task.ComputerSessionExpired = true
					case "unknown-version":
						task.Config.NativeWorkflowVersion = 99
					case "unsupported-control":
						view.Elements[0].Writable, view.Elements[0].Checkable = false, false
					}
					if fault == "legacy-host-new-observation" {
						view.Observation.ID, args["observation_id"] = "new-view", "new-view"
					}
					if fault == "legacy-task" {
						task.Config.NativeWorkflowVersion, task.Workflow = 0, nil
					}
					task.Steps[0].ToolResult = marshal(view)
					want := fault == "none" || fault == "new-reference" || fault == "legacy-host" || fault == "legacy-task"
					err := tools.ValidateWorkflowEdit(task, tool, json.RawMessage(marshal(args)))
					if (err == nil) != want {
						t.Fatalf("accepted=%t want=%t: %v", err == nil, want, err)
					}
					if fault == "application-only" && !errors.Is(err, errWorkflowControlExpectationRequired) {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestWorkflowEditCorrectDeclarationResolvesOnlyMatchingRepair(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", Driver: "windows-uia", OSSession: "os", ProcessGeneration: "process", Surface: "surface"}
	fixture := &declarationModelFixture{browserRunTools: &browserRunTools{}, manager: agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir()), view: computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "control", Identity: "stable", Writable: true}}}}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.NativeWorkflowVersion = true, 1
	cfg.ComputerSession, cfg.ComputerDriver, cfg.ComputerScope, cfg.ContextWindow = "session", target.Driver, "desktop", 32768
	cfg.NativeExpectations = []agents.OutcomeExpectation{{ID: "open", Title: "Opened", Kind: "application_open", ApplicationID: "fixture-editor"}}
	calls := 0
	runner := agents.NewRunner(fixture.manager, fixture, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		name, args := "", ""
		switch calls {
		case 1:
			name, args = "computer_verify_application", `{"application_id":"fixture-editor"}`
		case 2:
			name, args = "computer_replace_text", `{"observation_id":"view","element":"control","text":"Requested"}`
		case 3:
			name, args = "task_expectations", `{"expectations":[{"id":"other","title":"Other research","kind":"research"}]}`
		case 4:
			if !task.Checkpoint.WorkflowEditPending || task.Checkpoint.ProposalRepairs != 1 {
				t.Fatal("unrelated declaration cleared repair")
			}
			name, args = "task_expectations", `{"expectations":[{"id":"text","title":"Text","kind":"control_text_equals","target_id":"window","text":"Requested"}]}`
		case 5:
			name, args = "computer_observe", `{}`
		case 6:
			if !task.Checkpoint.WorkflowEditPending || task.Checkpoint.ProposalRepairs != 2 {
				t.Fatal("read reset repair")
			}
			name, args = "task_expectations", `{"expectations":[{"id":"text","title":"Text","kind":"control_text_equals","target_id":"window","element":"control","text":"Requested"}]}`
		default:
			if task.Checkpoint.WorkflowEditPending || task.Checkpoint.ProposalUnresolved || task.Checkpoint.ProposalRepairs != 0 {
				t.Fatal("matching declaration did not resolve repair")
			}
			return nil, context.Canceled
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	})
	defer runner.Shutdown(context.Background())
	task, err := runner.Create("Edit", "model", "fixture-actor", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.Continue(context.Background(), task.ID, "fixture-actor", "start", "", false)
	if !errors.Is(err, context.Canceled) || calls != 7 {
		t.Fatal(calls, err, got.Error)
	}
}

func TestWorkflowEditGuardPrecedesHostPrepareEvenWhenApproved(t *testing.T) {
	s, adapter, _, _, _ := compositionFixture(t)
	code, _ := s.browserHub.PairCode("alice")
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	host, token, err := s.browserHub.PairNative(code, "Fixture", target, 2, computer.ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	cfg := compositionConfig("windows-uia", true)
	cfg.ComputerSession, cfg.NativeWorkflowVersion = host.ID, 1
	cfg.NativeExpectations = []agents.OutcomeExpectation{{ID: "app", Title: "Opened", Kind: "application_open", ApplicationID: "app"}}
	runner := agents.NewRunner(s.agentManager, adapter, nil)
	task, err := runner.Create("edit", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "control", Identity: "stable", Writable: true}}}
	raw, _ := json.Marshal(view)
	if err = s.agentManager.AddTaskStep(task.ID, agents.Step{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(raw)}); err != nil {
		t.Fatal(err)
	}
	d, _ := adapter.Capability("computer_replace_text")
	args := json.RawMessage(`{"observation_id":"view","element":"control","text":"test"}`)
	for _, approved := range []bool{false, true} {
		e := agents.ToolExecution{Actor: "alice", RunID: task.ID, CallID: "edit", ExpectedCapability: &d, Approved: approved}
		var auth *agents.ToolAuthorizationError
		err = adapter.Authorize(context.Background(), "computer_replace_text", args, e)
		if !errors.As(err, &auth) || auth.Code != "workflow_control_expectation_required" {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err = adapter.ExecuteWithPolicy(context.Background(), "computer_replace_text", args, e); err == nil {
			t.Fatal("dispatch bypassed guard")
		}
		action, err := s.browserHub.Poll(token)
		if err != nil || action != nil {
			t.Fatal("guard contacted host", action, err)
		}
	}
}
