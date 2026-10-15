package server

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationalAnswerDoesNotFallIntoLegacyNativeCompletion(t *testing.T) {
	for _, scope := range []string{"", "desktop"} {
		t.Run(scope, func(t *testing.T) {
			s, adapter, _, _, calls := compositionFixture(t)
			cfg := compositionConfig("windows-uia", true)
			cfg.ComputerScope = scope
			cfg.NativeWorkflowVersion = 2
			r := agents.NewRunner(s.agentManager, adapter, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Hello!"}}}}, nil
			})
			task, err := r.Create("hello", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != agents.TaskCompleted || got.Result != "Hello!" || got.Workflow != nil || calls.Load() != 0 {
				t.Fatal("conversation rejected", got, err)
			}
		})
	}
}

func TestMatchedApplicationGuidanceAdvancesInsteadOfRepeatingLookup(t *testing.T) {
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerScope: "desktop", ComputerDriver: "windows-uia", ComputerSession: "session", NativeWorkflowVersion: 2}, Steps: []agents.Step{
		{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"host-obs","name":"OBS Studio"}]}`},
		{ID: 2, Type: "action", ToolName: applicationSearchTool, ToolArgs: `{"query":"OBS Studio","catalog_step":1}`, ToolResult: `{}`},
	}}
	adapter := &browserRunTools{}
	got := adapter.ProjectModelContext(task, nil)
	if len(got) == 0 || !strings.Contains(got[len(got)-1].StringContent(), "Do not repeat the same lookup") {
		t.Fatal("lookup guidance failed to advance", got)
	}
	task.Workflow = &agents.WorkflowEvidence{Checks: []agents.OutcomeCheck{{Expectation: agents.OutcomeExpectation{Kind: "application_open", ApplicationID: "host-obs"}}}}
	got = adapter.ProjectModelContext(task, nil)
	if !strings.Contains(got[len(got)-1].StringContent(), "already has a declared launch check") {
		t.Fatal("declaration guidance failed to advance", got)
	}
	task.Workflow.Checks[0].State = "verified"
	got = adapter.ProjectModelContext(task, nil)
	if !strings.Contains(got[len(got)-1].StringContent(), "report the actual result without another tool call") {
		t.Fatal("verified task still invited more work", got)
	}
}
