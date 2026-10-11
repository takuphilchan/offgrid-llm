package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func invalidAccessTools() *runTestTools {
	return &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error {
		return &ToolAuthorizationError{Code: "computer_invalid_access_request", Message: "Use app mode for installed applications."}
	}}
}

func accessProposalAnswer() *api.ChatCompletionResponse {
	return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "request_computer_access", Arguments: `{"mode":"browser","target":"Chrome","url":""}`}}}})
}

func newProposalTask(t *testing.T, r *Runner) *Task {
	t.Helper()
	cfg := DefaultAgentConfig()
	cfg.TaskFirst = true
	// Exercise repair limits, not the separate small-context admission guard.
	cfg.ContextWindow = 16384
	task, err := r.Create("open chrome", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestProposalRepairLimitAndExplicitRecovery(t *testing.T) {
	for _, action := range []string{"resume", "steer"} {
		t.Run(action, func(t *testing.T) {
			tools := invalidAccessTools()
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				return accessProposalAnswer(), nil
			})
			task := newProposalTask(t, r)
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 {
				t.Fatalf("repair did not stop: task=%+v calls=%d err=%v", got, calls, err)
			}
			if !CanSteerTask(got) || len(got.Checkpoint.Calls) != 0 || got.Checkpoint.ExecutingCall != "" || got.PendingApproval != nil || got.PendingInput != nil {
				t.Fatal("repair pause retained executable proposals or cannot be corrected")
			}
			for _, step := range got.Steps {
				if step.Type != "rejected" || !strings.Contains(step.ToolResult, `"effects":"none"`) {
					t.Fatalf("invalid rejection evidence: %+v", step)
				}
			}
			if _, err := r.Continue(context.Background(), task.ID, "bob", "resume", "", false); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("another actor resumed: %v", err)
			}
			if action == "steer" {
				got, err = r.Steer(task.ID, "alice", "correct-access", "Use application access for Chrome.")
				if err != nil || got.Checkpoint.ProposalRepairs != 0 {
					t.Fatalf("correction did not reset budget: %+v %v", got, err)
				}
			}
			got, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
			if err != nil || got.Status != TaskInterrupted || calls != 8 || tools.calls.Load() != 0 {
				t.Fatalf("explicit resume did not obtain a fresh bounded attempt: calls=%d err=%v task=%+v", calls, err, got)
			}
		})
	}
}

func TestProposalRepairBudgetSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	tools := invalidAccessTools()
	modelCalls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		if modelCalls == 3 {
			return nil, context.Canceled
		}
		return accessProposalAnswer(), nil
	})
	task := newProposalTask(t, r)
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !errors.Is(err, context.Canceled) || got.Status != TaskInterrupted || got.Checkpoint.ProposalRepairs != 2 {
		t.Fatalf("unexpected interrupted checkpoint: %+v %v", got, err)
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	modelCalls = 0
	r = NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		return accessProposalAnswer(), nil
	})
	got, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || got.ErrorCode != proposalRepairLimitCode || modelCalls != 2 || tools.calls.Load() != 0 {
		t.Fatalf("restart reset budget: calls=%d task=%+v err=%v", modelCalls, got, err)
	}
}

func TestProposalRepairDoesNotRepairDenialsOrExecutionErrors(t *testing.T) {
	for _, code := range []string{"computer_prohibited_action", "computer_scope_violation", "computer_approval_invalid", "computer_connection_lost", "ordinary_denial", "execution_error"} {
		t.Run(code, func(t *testing.T) {
			tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error {
				if code == "execution_error" {
					return nil
				}
				if code == "ordinary_denial" {
					return capabilities.ErrDenied
				}
				return &ToolAuthorizationError{Code: code, Message: "Access denied."}
			}, execute: func(string, json.RawMessage, ToolExecution) (string, error) {
				// Even this typed error is not repairable after the intent boundary.
				return "", &ToolAuthorizationError{Code: "computer_invalid_access_request", Message: "uncertain provider outcome"}
			}}
			modelCalls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				modelCalls++
				return accessProposalAnswer(), nil
			})
			task := newProposalTask(t, r)
			got, _ := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			want, executions := TaskFailed, int32(0)
			if code == "execution_error" {
				want, executions = TaskUncertain, 1
			}
			if got.Status != want || modelCalls != 1 || tools.calls.Load() != executions || got.Checkpoint.ProposalRepairs != 0 {
				t.Fatalf("unsafe repair: task=%+v model=%d executions=%d", got, modelCalls, tools.calls.Load())
			}
			if _, err := r.Continue(context.Background(), got.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
				t.Fatalf("terminal/uncertain task resumed: %v", err)
			}
		})
	}
}

func TestProposalRepairAbandonsDependentBatch(t *testing.T) {
	tools := invalidAccessTools()
	modelCalls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		if modelCalls == 1 {
			answer := accessProposalAnswer()
			answer.Choices[0].Message.ToolCalls = append(answer.Choices[0].Message.ToolCalls, api.ToolCall{Function: api.FunctionCall{Name: "write_file", Arguments: `{}`}})
			return answer, nil
		}
		if modelCalls == 2 && (len(task.Checkpoint.Calls) != 0 || !strings.Contains(messages[len(messages)-1].StringContent(), "tool_proposal_abandoned")) {
			t.Fatal("dependent call was not abandoned")
		}
		return runAnswer(api.ChatMessage{Content: "No changes made."}), nil
	})
	task := newProposalTask(t, r)
	task, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.ErrorCode != proposalRepairLimitCode || tools.calls.Load() != 0 || len(task.Steps) != 5 || modelCalls != 4 {
		t.Fatalf("invalid batch dispatched: %+v %v", task, err)
	}
}

func TestProposalRepairResetAfterProgress(t *testing.T) {
	tools := invalidAccessTools()
	tools.authorize = func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name == "progress" {
			return nil
		}
		return &ToolAuthorizationError{Code: "computer_invalid_access_request", Message: "Correct access."}
	}
	tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) { return "progress", nil }
	modelCalls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		if modelCalls == 4 {
			return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "progress", Arguments: `{}`}}}}), nil
		}
		return accessProposalAnswer(), nil
	})
	task := newProposalTask(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, err := r.Continue(ctx, task.ID, "alice", "start", "", false)
	if err != nil || task.ErrorCode != proposalRepairLimitCode || modelCalls != 8 || tools.calls.Load() != 1 {
		t.Fatalf("progress did not reset consecutive budget: %+v %v model=%d", task, err, modelCalls)
	}
}

func TestProposalRepairUnresolvedSurvivesResumeAndRestart(t *testing.T) {
	dir := t.TempDir()
	tools := invalidAccessTools()
	calls := 0
	caller := func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return accessProposalAnswer(), nil
		}
		return runAnswer(api.ChatMessage{Content: "Please submit the tool request yourself."}), nil
	}
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, caller)
	task := newProposalTask(t, r)
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskInterrupted || !got.Checkpoint.ProposalUnresolved || calls != 4 {
		t.Fatalf("not paused: %+v %v", got, err)
	}
	if err = r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	r = NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, caller)
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	got, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || got.Status != TaskInterrupted || !got.Checkpoint.ProposalUnresolved || calls != 8 || tools.calls.Load() != 0 || got.Result != "" {
		t.Fatalf("resume fabricated completion: %+v %v calls=%d", got, err, calls)
	}
}

func TestProposalRepairUnrelatedReadCannotResolveAccess(t *testing.T) {
	tools := invalidAccessTools()
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name != "current_time" {
			t.Fatalf("unexpected dispatch: %s", name)
		}
		return `{"time":"test-only clock"}`, nil
	}
	tools.authorize = func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name == "current_time" {
			return nil
		}
		return &ToolAuthorizationError{Code: "computer_invalid_access_request", Message: "Correct access."}
	}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return accessProposalAnswer(), nil
		}
		if calls == 2 {
			return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "current_time", Arguments: `{}`}}}}), nil
		}
		return runAnswer(api.ChatMessage{Content: "Chrome was opened."}), nil
	})
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	task := newProposalTask(t, r)
	task, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.Result != "" || tools.calls.Load() != 1 || !task.Checkpoint.ProposalUnresolved {
		t.Fatalf("unrelated read cleared failed access: %+v %v", task, err)
	}
}
