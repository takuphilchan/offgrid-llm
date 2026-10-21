package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func artifactDecision(req taskArtifactRequest) *api.ChatCompletionResponse {
	raw, _ := json.Marshal(req)
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "fixture", Type: "function", Function: api.FunctionCall{Name: taskArtifactToolName, Arguments: string(raw)}}}}}}}
}

func TestArtifactSchemaMatchesFilenameContract(t *testing.T) {
	for _, tc := range []struct {
		name, format, content string
		valid                 bool
	}{
		{"search_results", "text", "draft", false},
		{"search_results.md", "text", "draft", false},
		{"../draft.txt", "text", "draft", false},
		{"C:\\draft.txt", "text", "draft", false},
		{".txt", "text", "draft", false},
		{"draft.txt", "text", "", false},
		{"draft.txt", "text", "café 日本語", true},
		{"draft.md", "markdown", "# Draft", true},
		{"draft.json", "json", "{}", true},
		{"draft.csv", "csv", "a,b\n1,2", true},
	} {
		t.Run(tc.name+tc.format, func(t *testing.T) {
			err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{taskArtifactTool()}}, artifactDecision(taskArtifactRequest{tc.name, tc.format, tc.content}))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestArtifactValidationFeedbackIsSpecificAndPrivate(t *testing.T) {
	for _, tc := range []struct {
		req  taskArtifactRequest
		want string
	}{
		{taskArtifactRequest{"search_results", "text", "private payload"}, ".txt"},
		{taskArtifactRequest{"../private.txt", "text", "private payload"}, "plain filename"},
		{taskArtifactRequest{"draft.txt", "text", ""}, "content is empty"},
		{taskArtifactRequest{"draft.txt", "text", strings.Repeat("日", 44000)}, "131072 UTF-8 bytes"},
	} {
		raw, _ := json.Marshal(tc.req)
		_, _, err := parseTaskArtifact(raw)
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "private") {
			t.Fatalf("wrong feedback: %v", err)
		}
	}
}

// Real catalog, authorization, persistence and strict adapter; generation is
// synthetic. This executes only private artifacts in disposable test storage.
func TestArtifactRejectionRecoversBeforeEffects(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(map[bool]string{false: "authorization", true: "adapter"}[strict], func(t *testing.T) {
			store, _ := artifacts.NewStore(t.TempDir())
			manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			s := &Server{artifactStore: store, agentManager: manager}
			tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
			calls := 0
			r := agents.NewRunner(manager, tools, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, offered []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				var response *api.ChatCompletionResponse
				switch calls {
				case 1:
					response = artifactDecision(taskArtifactRequest{"search_results", "text", "Synthetic draft, not researched findings."})
				case 2:
					if len(taskArtifacts(task)) != 0 || task.Checkpoint.ExecutingCall != "" || len(task.Checkpoint.Calls) != 0 {
						t.Fatal("rejection caused effects or left queued work")
					}
					response = artifactDecision(taskArtifactRequest{"search_results.txt", "text", "Synthetic draft, not researched findings."})
				default:
					response = &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Saved a private draft. No research was performed."}}}}
				}
				if strict {
					return response, validateAgentDecision(task, &api.ChatCompletionRequest{Tools: offered}, response)
				}
				return response, nil
			})
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst = true
			cfg.ContextWindow = 16384
			task, err := r.Create("Save a private draft", "model", "local-admin", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
			if err != nil || got.Status != agents.TaskCompleted || calls != 3 || len(taskArtifacts(got)) != 1 {
				t.Fatalf("status=%s code=%s calls=%d err=%v", got.Status, got.ErrorCode, calls, err)
			}
		})
	}
}

func TestArtifactMenuPreservesRequestedApplicationWork(t *testing.T) {
	store, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{artifactStore: store}
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	cfg := compositionConfig("windows-uia", true)
	cfg.ComputerScope = "desktop"
	task := &agents.Task{Config: cfg}
	if !availableNames(tools.ToolsForTask(task))[taskArtifactToolName] {
		t.Fatal("preattached grant gated private drafts")
	}
	task.LastAccess = &agents.InputRequest{Kind: "computer", CallID: "access", Target: "requested application"}
	if availableNames(tools.ToolsForTask(task))[taskArtifactToolName] {
		t.Fatal("artifact offered before requested target inspection")
	}
	// Metadata, rejected calls and past-session observations are not progress.
	task.Steps = []agents.Step{{Type: "action", ToolName: "computer_observe"}, {Type: "rejected", ToolName: "computer_observe"}, {Type: "task_metadata", ToolName: "task_plan"}}
	task.Config.ComputerStepOffset = 1
	if availableNames(tools.ToolsForTask(task))[taskArtifactToolName] {
		t.Fatal("old/unexecuted evidence satisfied inspection")
	}
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "computer_observe"})
	if !availableNames(tools.ToolsForTask(task))[taskArtifactToolName] {
		t.Fatal("current inspection did not restore artifact tool")
	}
}

func TestArtifactPendingWorkGuardAlsoChecksPersistedProposals(t *testing.T) {
	store, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	s := &Server{artifactStore: store, agentManager: manager}
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	r := agents.NewRunner(manager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		response := artifactDecision(taskArtifactRequest{})
		response.Choices[0].Message.ToolCalls[0].Function = api.FunctionCall{Name: "request_computer_access", Arguments: `{"mode":"app","target":"Requested browser","url":""}`}
		return response, nil
	})
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 16384
	task, err := r.Create("Inspect the requested application", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.PendingInput == nil {
		t.Fatalf("access request failed: %+v %v", task, err)
	}
	cfg = task.Config
	cfg.ComputerSession, cfg.ComputerDriver, cfg.ComputerScope, cfg.ComputerApprovalMode = "synthetic", "windows-uia", "desktop", "full_task"
	task, err = r.ResolveComputerInput(task.ID, task.Actor, task.PendingInput.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := taskArtifactDescriptor()
	for _, name := range []string{"search_results", "search_results.txt"} {
		raw, _ := json.Marshal(taskArtifactRequest{name, "text", "request only, not researched findings"})
		err := tools.Authorize(context.Background(), taskArtifactToolName, raw, agents.ToolExecution{RunID: task.ID, Actor: task.Actor, ExpectedCapability: &descriptor})
		var rejection *agents.ToolAuthorizationError
		if !errors.As(err, &rejection) || rejection.Code != "task_artifact_work_pending" {
			t.Fatalf("stale proposal bypassed progress gate: %v", err)
		}
	}
	if len(taskArtifacts(task)) != 0 {
		t.Fatal("access caused artifact write")
	}
}
