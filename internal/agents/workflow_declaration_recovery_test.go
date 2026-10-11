package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Regression for the installed-model Notepad test: a control predicate was
// proposed before a window existed, without an element. Never accept invented
// targets, but give field-specific, actionable feedback instead of a dead end.
const prematureControlExpectation = `{"expectations":[{"id":"text","title":"Confirm editor text","kind":"control_text_equals","target_id":"1","text":"OffGrid test — café 日本語 12345"}]}`

func TestWorkflowDeclarationErrorsAreActionableAndBounded(t *testing.T) {
	for _, firstLaunch := range []bool{false, true} {
		t.Run(map[bool]string{false: "declaration", true: "rejected_launch"}[firstLaunch], func(t *testing.T) {
			tools := &workflowTestTools{}
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls > 1 && !(firstLaunch && calls == 2) {
					feedback := messages[len(messages)-1].StringContent()
					if !strings.Contains(feedback, "element") || !strings.Contains(feedback, "application_open") {
						t.Fatalf("not actionable: %s", feedback)
					}
				}
				if firstLaunch && calls == 1 {
					return taskCall("computer_open_application", `{"application_id":"editor"}`), nil
				}
				return taskCall("task_expectations", prematureControlExpectation), nil
			})
			task, err := r.Create("Open editor and type Unicode in a new document", "model", "alice", workflowConfig())
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 {
				t.Fatalf("unbounded declaration repair: calls=%d status=%s code=%s err=%v", calls, got.Status, got.ErrorCode, err)
			}
			if len(got.Workflow.Checks) != 0 || got.Checkpoint.ExecutingCall != "" || len(got.Checkpoint.Calls) != 0 {
				t.Fatal("invalid declaration retained authority")
			}
			for _, step := range got.Steps {
				if step.ToolName == "task_expectations" && step.ToolArgs != prematureControlExpectation {
					var a, b any
					_ = json.Unmarshal([]byte(step.ToolArgs), &a)
					_ = json.Unmarshal([]byte(prematureControlExpectation), &b)
					x, _ := json.Marshal(a)
					y, _ := json.Marshal(b)
					if string(x) != string(y) {
						t.Fatal("lost rejected arguments from saved history")
					}
				}
			}
		})
	}
}

func TestWorkflowDeclarationRecoveryCanProceedWithoutChangingExpectation(t *testing.T) {
	tools := &workflowTestTools{}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name != "computer_open_application" {
			t.Fatalf("unexpected tool: %s", name)
		}
		return `{}`, nil
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		switch calls {
		case 1:
			return taskCall("task_expectations", prematureControlExpectation), nil
		case 2:
			return taskCall("task_expectations", `{"expectations":[{"id":"open","title":"Editor ready","kind":"application_open","application_id":"editor"}]}`), nil
		case 3:
			if task.Checkpoint.ProposalUnresolved || task.Checkpoint.ProposalRepairs != 0 {
				t.Fatal("successful declaration did not resolve repair")
			}
			return taskCall("computer_open_application", `{"application_id":"editor"}`), nil
		default:
			return nil, context.Canceled // no fabricated readiness/completion
		}
	})
	task, err := r.Create("Open editor", "model", "alice", workflowConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if calls != 4 || tools.calls.Load() != 1 || len(got.Workflow.Checks) != 1 || got.Workflow.Checks[0].State != "pending" {
		t.Fatal("repair lost checks or duplicated dispatch")
	}
}

func TestWorkflowDeclarationReadsAndRestartDoNotResetRepairBudget(t *testing.T) {
	dir := t.TempDir()
	tools := &workflowTestTools{}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) { return `{}`, nil }
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 4 {
			return nil, context.Canceled
		}
		if calls == 2 {
			return taskCall("computer_observe", `{}`), nil
		}
		return taskCall("task_expectations", prematureControlExpectation), nil
	})
	task, err := r.Create("Open editor", "model", "alice", workflowConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if got.Checkpoint.ProposalRepairs != 2 || !got.Checkpoint.ProposalUnresolved {
		t.Fatal("read reset invalid declaration")
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls = 0
	r = NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return taskCall("task_expectations", prematureControlExpectation), nil
	})
	got, ok := r.manager.GetTask(task.ID)
	if !ok || got.Checkpoint.ProposalRepairs != 2 || !got.Checkpoint.ProposalUnresolved || !got.ComputerSessionExpired {
		t.Fatal("restart lost repair budget or restored old desktop authority")
	}
	_, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if !errors.Is(err, ErrRunConflict) || calls != 0 || tools.calls.Load() != 1 {
		t.Fatalf("restart reused expired desktop authority: calls=%d err=%v", calls, err)
	}
}

func TestWorkflowControlEditRepairSurvivesReadsAndUnrelatedDeclarations(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	tools := &workflowTestTools{}
	tools.authorize = func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name == "computer_replace_text" {
			return &ToolAuthorizationError{Code: "workflow_control_expectation_required", Message: "Declare the matching check before editing."}
		}
		return nil
	}
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name != "computer_observe" {
			t.Fatal("unguarded mutation", name)
		}
		return `{}`, nil
	}
	calls := 0
	r := NewRunner(m, tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 || calls == 4 || calls == 6 || calls == 8 {
			return taskCall("computer_replace_text", `{"observation_id":"view","element":"control","text":"Requested"}`), nil
		}
		if calls == 3 {
			return taskCall("task_expectations", `{"expectations":[{"id":"unrelated","title":"Unrelated research","kind":"research"}]}`), nil
		}
		if calls == 5 {
			return taskCall("task_expectations", prematureControlExpectation), nil
		}
		if calls == 7 {
			t.Fatal("repair limit bypassed")
		}
		if calls > 1 && (!task.Checkpoint.WorkflowEditPending || task.Checkpoint.ProposalRepairs == 0) {
			t.Fatal("lost pending edit repair")
		}
		return taskCall("computer_observe", `{}`), nil
	})
	cfg := workflowConfig()
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Opened", Kind: "application_open", ApplicationID: "app"}}
	task, err := r.Create("Edit", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.AddTaskStep(task.ID, Step{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: `{"observation":{"id":"view"}}`}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 6 || tools.calls.Load() != 1 {
		t.Fatalf("calls=%d task=%s/%s err=%v message=%s", calls, got.Status, got.ErrorCode, err, got.Error)
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, ok := reloaded.GetTask(task.ID)
	if !ok || !saved.Checkpoint.WorkflowEditPending || saved.Checkpoint.ProposalRepairs != 4 || !saved.ComputerSessionExpired {
		t.Fatal("restart lost repair or restored authority")
	}
}

func TestWorkflowDeclarationGuidanceRespectsSelectedWindowScope(t *testing.T) {
	for _, driver := range []string{"macos-accessibility", "linux-atspi", "windows-uia"} {
		task := &Task{Config: workflowConfig()}
		task.Config.ComputerDriver, task.Config.ComputerScope = driver, "window"
		guidance := workflowDeclarationGuidance(task)
		for _, tool := range runtimeTools(task) {
			if tool.Function.Name == "task_expectations" {
				guidance += tool.Function.Description
			}
		}
		if strings.Contains(guidance, "computer_applications") || strings.Contains(guidance, "computer_open_application") || !strings.Contains(guidance, "control_text_equals") {
			t.Fatal(driver, guidance)
		}
	}
}

func TestWorkflowToolSchemaRequiresControlReferencesByKind(t *testing.T) {
	for _, name := range []string{"computer_replace_text", "computer_set_checked"} {
		tool := workflowTool()
		task := &Task{Checkpoint: &Checkpoint{WorkflowEditPending: true}, Steps: []Step{{Type: "rejected", ToolName: name, ToolResult: `{"error":"workflow_control_expectation_required"}`}}}
		constrainWorkflowRepairTool(&tool, task)
		item := tool.Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
		required := item["required"].([]string)
		if len(required) != 6 || required[0] != "id" || required[3] != "target_id" || required[4] != "element" {
			t.Fatal("schema does not describe the enforced control contract", required)
		}
		fresh := workflowTool().Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
		if len(fresh["required"].([]string)) != 3 {
			t.Fatal("repair modified other tasks' schema")
		}
	}
}
