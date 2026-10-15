package agents

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

type assessedTestTools struct {
	runTestTools
	states map[string]string
}

func TestVerifiedLaunchCannotCompleteUnfinishedDocumentPlan(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &assessedTestTools{states: map[string]string{"open": "verified"}}
	calls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Done"}), nil
	})
	cfg := workflowConfig()
	cfg.MaxIterations = 20
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open editor", Kind: "application_open", ApplicationID: "app"}}
	task, err := r.Create("Open and write the requested document", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(saved *Task) error {
		saved.Plan = []PlanItem{{ID: "write", Title: "Write the requested document", State: "pending"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || done.Status != TaskInterrupted || done.ErrorCode != "workflow_completion_repair_limit" || calls != 4 || tools.calls.Load() != 0 {
		t.Fatalf("unfinished plan falsely completed: status=%s code=%s calls=%d err=%v", done.Status, done.ErrorCode, calls, err)
	}
}

func TestWorkflowUncertainMutationNeverEntersCompletionRepair(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &workflowTestTools{}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name == "computer_observe" {
			return `{}`, nil
		}
		return "", errors.New("lost acknowledgment")
	}
	calls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		name := "computer_activate"
		if calls == 1 {
			name = "computer_observe"
		}
		return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: name, Arguments: `{}`}}}}), nil
	})
	cfg := workflowConfig()
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "submission", Title: "Submit once", Kind: "submission"}}
	task, err := r.Create("submit", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err == nil || done.Status != TaskUncertain || done.Checkpoint.CompletionRepairs != 0 || calls != 2 || tools.calls.Load() != 2 {
		t.Fatal(done, err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("uncertain replay", err)
	}
	if calls != 2 || tools.calls.Load() != 2 {
		t.Fatal("replayed action")
	}
}

func (a *assessedTestTools) AssessWorkflow(t *Task) []OutcomeCheck {
	checks := append([]OutcomeCheck(nil), t.Workflow.Checks...)
	for i := range checks {
		checks[i].State = "pending"
		if s := a.states[checks[i].Expectation.ID]; s != "" {
			checks[i].State = s
		}
	}
	return checks
}
func TestWorkflowCompletionRepairBudgetSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	tools := &assessedTestTools{states: map[string]string{"open": "verified"}}
	calls := 0
	caller := func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", Content: "Done"}, FinishReason: "stop"}}}, nil
	}
	r := NewRunner(m, tools, caller)
	cfg := workflowConfig()
	cfg.MaxIterations = 20
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open", Kind: "application_open", ApplicationID: "app"}, {ID: "text", Title: "Text", Kind: "control_text_equals", TargetID: "target", Element: "control", Text: func() *string { s := "hello"; return &s }()}}
	task, err := r.Create("Open and write", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || done.Status != TaskInterrupted || done.ErrorCode != "workflow_completion_repair_limit" || calls != 4 || done.Checkpoint.CompletionRepairs != 3 || tools.calls.Load() != 0 {
		t.Fatalf("repair failed: %+v %v calls=%d", done, err, calls)
	}
	recovered := NewManagerWithPersistence(nil, nil, nil, dir)
	saved, _ := recovered.GetTask(task.ID)
	if saved.Checkpoint.CompletionRepairs != 3 || len(saved.Workflow.Checks) != 2 || saved.ID != task.ID {
		t.Fatal("lost repair checkpoint")
	}
	replay, err := recovered.ReplayTask(context.Background(), task.ID, "alice", 0)
	if err != nil || len(replay.Events) == 0 {
		t.Fatal(err)
	}
	if replay.Events[len(replay.Events)-1].VerificationState != "pending" {
		t.Fatal("event lost committed assessment")
	}
	if _, err = recovered.ReplayTask(context.Background(), task.ID, "bob", 0); err == nil {
		t.Fatal("foreign replay")
	}
	resumed := NewRunner(recovered, tools, caller)
	if _, err = resumed.Continue(context.Background(), task.ID, "alice", "resume", "", false); err == nil {
		t.Fatal("restart must require expired host access recovery")
	}
}

func TestWorkflowProgressCannotResetBudgetByInvalidatingSameCheck(t *testing.T) {
	tools := &assessedTestTools{states: map[string]string{"open": "verified"}}
	r := NewRunner(nil, tools, nil)
	task := &Task{Config: workflowConfig(), Checkpoint: &Checkpoint{CompletionRepairs: 3}}
	_ = initializeWorkflow(task)
	_ = declareExpectations(task, []OutcomeExpectation{{ID: "open", Title: "Open", Kind: "application_open", ApplicationID: "app"}}, "model_proposed")
	r.assessWorkflow(task)
	if task.Checkpoint.CompletionRepairs != 0 {
		t.Fatal("new progress did not reset")
	}
	task.Checkpoint.CompletionRepairs = 2
	tools.states["open"] = "pending"
	r.assessWorkflow(task)
	tools.states["open"] = "verified"
	r.assessWorkflow(task)
	if task.Checkpoint.CompletionRepairs != 2 {
		t.Fatal("repeated same proof reset budget")
	}
}

func TestUnsupportedVerificationAllowsExplicitPartialResult(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &assessedTestTools{states: map[string]string{"open": "verified", "research": "unavailable"}}
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", Content: "Opened. Research is unverified."}, FinishReason: "stop"}}}, nil
	})
	cfg := workflowConfig()
	cfg.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open", Kind: "application_open", ApplicationID: "app"}, {ID: "research", Title: "Research", Kind: "research"}}
	task, err := r.Create("Open and research", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || done.Status != TaskCompleted || done.Workflow.State != "partial" || done.Checkpoint.CompletionRepairs != 0 {
		t.Fatalf("%+v %v", done, err)
	}
	data, _ := json.Marshal(done)
	if !json.Valid(data) {
		t.Fatal("invalid snapshot")
	}
}
