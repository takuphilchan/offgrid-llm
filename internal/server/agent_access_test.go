package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAccessProposalValidation(t *testing.T) {
	for _, args := range []string{
		`{"mode":"browser","target":"Chrome","url":""}`,
		`{"mode":"browser","target":"Chrome","url":"file:///etc/passwd"}`,
		`{"mode":"browser","target":"site","url":"https://user:secret@example.com"}`,
		`{"mode":"app","target":"Chrome","url":"https://example.com"}`,
		`{"mode":"app","target":"Chrome","url":"","approved":true}`,
		`{"mode":"app","target":"Chrome","url":""} {}`,
		`{"mode":"app","target":"Chrome","url":""} trailing`,
		`{"mode":"desktop","target":"all","url":""}`,
		`{"mode":"app","target":"   ","url":""}`,
		`{"mode":"app","target":123,"url":""}`,
		`null`,
	} {
		t.Run(args, func(t *testing.T) {
			err := computerAccessRequest(json.RawMessage(args))
			var safe *agents.ToolAuthorizationError
			var consent *agents.InputRequired
			if !errors.As(err, &safe) || safe.Code != "computer_invalid_access_request" || errors.As(err, &consent) || errors.Is(err, capabilities.ErrDenied) {
				t.Fatalf("expected recoverable validation, not permission/consent: %v", err)
			}
			if !strings.Contains(safe.Message, "mode app") || !strings.Contains(safe.Message, "HTTPS") || strings.Contains(safe.Message, "user:secret") {
				t.Fatalf("missing safe corrective guidance: %s", safe.Message)
			}
		})
	}
	for _, args := range []string{`{"mode":"app","target":"Chrome","url":""}`, `{"mode":"browser","target":"Docs","url":"https://example.com/docs"}`} {
		var input *agents.InputRequired
		if err := computerAccessRequest(json.RawMessage(args)); !errors.As(err, &input) || input.Request.Kind != "computer" {
			t.Fatalf("valid proposal must request consent, not execute: %v", err)
		}
	}
}

func TestPreAccessPromptUsesTaskWideDesktopWorkflow(t *testing.T) {
	if !strings.Contains(taskFirstPrompt, "task-wide desktop access") || !strings.Contains(taskFirstPrompt, "without another access request for each window") {
		t.Fatal("pre-access guidance must describe the current desktop grant")
	}
	if strings.Contains(taskFirstPrompt, "releases the old session") || strings.Contains(taskFirstPrompt, "request the next access") {
		t.Fatal("pre-access guidance must not restore the retired per-window workflow")
	}
}

func TestAccessProposalRunnerRepairsChromeBeforeConsent(t *testing.T) {
	s := taskFirstServer(t)
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	modelCalls := 0
	r := agents.NewRunner(s.agentManager, tools, func(_ context.Context, _ *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		args := `{"mode":"browser","target":"Chrome","url":""}`
		if modelCalls == 2 {
			if feedback := messages[len(messages)-1].StringContent(); !strings.Contains(feedback, "computer_invalid_access_request") || !strings.Contains(feedback, "mode app") {
				t.Fatalf("missing repair feedback: %s", feedback)
			}
			args = `{"mode":"app","target":"Chrome","url":""}`
		} else if modelCalls > 2 {
			t.Fatal("model continued without local consent")
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: computerAccessToolName, Arguments: args}}}}}}}, nil
	})
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 8192
	task, err := r.Create("open chrome", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := task.ID
	task, err = r.Continue(context.Background(), id, "local-admin", "start", "", false)
	if err != nil || task.ID != id || task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.Mode != "app" || task.PendingInput.Target != "Chrome" {
		t.Fatalf("Chrome correction did not reach consent in original task: %+v %v", task, err)
	}
	if len(task.Steps) != 1 || task.Steps[0].Type != "rejected" || task.Config.ComputerSession != "" || task.Checkpoint.ExecutingCall != "" || task.PendingApproval != nil {
		t.Fatal("access correction created action/approval/session authority")
	}
}

func TestAccessProposalRunnerRejectsInstructionsAfterInvalidChrome(t *testing.T) {
	s := taskFirstServer(t)
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	calls := 0
	r := agents.NewRunner(s.agentManager, tools, func(_ context.Context, _ *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		message := api.ChatMessage{Role: "assistant", Content: `Please submit {"mode":"app","target":"Chrome","url":""} yourself.`}
		finish := "stop"
		if calls == 1 {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{Function: api.FunctionCall{Name: computerAccessToolName, Arguments: `{"mode":"app","target":"Chrome","url":"https://www.google.com/"}`}}}
			finish = "tool_calls"
		} else if calls > 2 && !strings.Contains(messages[len(messages)-1].StringContent(), "Correct the proposal yourself") {
			t.Fatal("model not instructed to repair its own proposal")
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
	})
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 16384
	task, err := r.Create("open google chrome and search tourism", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.Status != agents.TaskInterrupted || task.ErrorCode != "tool_proposal_repair_limit" || calls != 4 || task.Result != "" || task.Checkpoint.ExecutingCall != "" {
		t.Fatalf("false completion: %+v calls=%d err=%v", task, calls, err)
	}
}
