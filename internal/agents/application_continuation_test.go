package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestPendingApplicationEvidenceSurvivesCompactionAndRestart(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ComputerScope, cfg.ComputerSession = "desktop", "host"
	cfg.ContextWindow, cfg.MaxTokens = 4096, 512
	task, err := r.Create("open Edge and search tourism", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(x *Task) error { x.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	task.Config.ComputerStepOffset = 1
	task.Steps = []Step{
		{Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"previous-session"}`},
		{Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"edge"}`, ToolResult: `{"actions":[{"outcome":"dispatched"}]}`},
	}
	prefix := task.Checkpoint.Messages[0].StringContent()
	for i := 0; i < 6; i++ {
		task.Checkpoint.Messages = append(task.Checkpoint.Messages,
			api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "read", Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}},
			api.ChatMessage{Role: "tool", ToolCallID: "read", Content: strings.Repeat("evidence ", 200)})
	}
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ContextArchive) == 0 || messages[0].StringContent() != prefix || !strings.Contains(messages[len(messages)-3].StringContent(), `["edge"]`) || messages[len(messages)-1].Role != "tool" {
		t.Fatal("launch evidence lost or prompt prefix rewritten")
	}
	if err = r.persist(task); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	loaded, ok := restored.GetTask(task.ID)
	if !ok || strings.Join(PendingApplicationLaunches(loaded), ",") != "edge" {
		t.Fatal("restart forgot the dispatched launch")
	}
	for _, result := range []string{`{"verified":false,"check":"application_open","application_id":"edge"}`, `{"verified":true,"check":"application_open","application_id":"chrome"}`} {
		loaded.Steps = append(loaded.Steps, Step{Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"edge"}`, ToolResult: result})
		if len(PendingApplicationLaunches(loaded)) != 1 {
			t.Fatal("unmatched verification cleared pending launch")
		}
	}
	loaded.Steps = append(loaded.Steps, Step{Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"edge"}`, ToolResult: `{"verified":true,"check":"application_open","application_id":"edge"}`})
	if len(PendingApplicationLaunches(loaded)) != 0 {
		t.Fatal("matching verification did not resolve launch")
	}
	if strings.Join(VerifiedApplications(loaded), ",") != "edge" {
		t.Fatal("verified progress was lost")
	}
	if err = r.persist(loaded); err != nil {
		t.Fatal(err)
	}
	restored = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	loaded, ok = restored.GetTask(task.ID)
	if !ok || strings.Join(VerifiedApplications(loaded), ",") != "edge" {
		t.Fatal("restart forgot verified application progress")
	}
	loaded.Steps = append(loaded.Steps, Step{Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"edge"}`, ToolResult: `{"verified":false,"check":"application_open","application_id":"edge"}`})
	if len(VerifiedApplications(loaded)) != 0 {
		t.Fatal("negative readiness did not clear progress")
	}
}

func TestTaskStepEvidencePagingIsBoundedAndTaskLocal(t *testing.T) {
	text := strings.Repeat("界🧭", 1000)
	task := &Task{Steps: []Step{{ID: 1, Type: "action", ToolResult: text}}}
	offset := 0
	reconstructed := ""
	for offset < len(text) {
		page, err := contextPage(task, "step:1", offset)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Content string `json:"content"`
			Next    int    `json:"next_offset"`
		}
		if json.Unmarshal([]byte(page), &result) != nil || len(result.Content) > 1024 || result.Next <= offset {
			t.Fatal("invalid page")
		}
		reconstructed += result.Content
		offset = result.Next
	}
	if reconstructed != text {
		t.Fatal("evidence changed")
	}
	for _, ref := range []string{"step:0", "step:2", "step:-1", "step:invalid"} {
		if _, err := contextPage(task, ref, 0); err == nil {
			t.Fatal("invalid reference accepted", ref)
		}
	}
	if _, err := contextPage(&Task{}, "step:1", 0); err == nil {
		t.Fatal("foreign evidence accepted")
	}
	if _, err := contextPage(task, "step:1", 1); err == nil {
		t.Fatal("split Unicode accepted")
	}
}

func TestUnverifiedLaunchRepairIsBoundedBeforeDispatch(t *testing.T) {
	tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error {
		return &ToolAuthorizationError{Code: "computer_application_verification_required", Message: "Verify the previous launch."}
	}}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return taskCall("computer_open_application", `{"application_id":"edge"}`), nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ComputerScope, cfg.ComputerSession = "desktop", "host"
	task, err := r.Create("open Edge", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 {
		t.Fatalf("unbounded or dispatched: calls=%d task=%+v err=%v", calls, got, err)
	}
}

func TestDiscoveryCannotResetUnverifiedLaunchRepairBudget(t *testing.T) {
	tools := &runTestTools{
		authorize: func(name string, _ json.RawMessage, _ ToolExecution) error {
			if name == "computer_open_application" {
				return &ToolAuthorizationError{Code: "computer_application_verification_required", Message: "Verify the previous launch."}
			}
			return nil
		},
		execute: func(string, json.RawMessage, ToolExecution) (string, error) { return `{}`, nil },
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls%2 == 0 {
			return taskCall("computer_applications", `{}`), nil
		}
		return taskCall("computer_open_application", `{"application_id":"edge"}`), nil
	})
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ComputerScope, cfg.ComputerSession = "desktop", "host"
	task, err := r.Create("open Edge", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 7 || tools.calls.Load() != 3 {
		t.Fatalf("discovery reset repair or a launch dispatched: calls=%d task=%+v err=%v", calls, got, err)
	}
}
