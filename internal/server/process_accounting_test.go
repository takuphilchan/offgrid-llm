package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Side effects are restricted to a child of this test binary and disposable
// host/journal directories. No installed state, desktop apps or network access.
func TestProcessAccountingThroughRunnerAndAuthorizedHost(t *testing.T) {
	f := newConversationFixture(t)
	root := t.TempDir()
	b := execution.Binding{Actor: "local-admin", Workspace: f.s.workspaceID, Host: "budget-host", Session: "budget-session", Grant: "budget-grant", Revision: 1, Job: "supervisor"}
	hostConfig := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true, Processes: true}}
	session, err := execution.OpenSession(hostConfig)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := execution.OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := &execution.Worker{Session: session, Journal: journal, Processes: execution.NewProcessManager(session), Token: strings.Repeat("b", 64), AuthorizeProcess: execution.TrustedProcessPolicy}
	endpoint := execution.NewLocalEndpoint(worker)
	t.Cleanup(func() { endpoint.Close(); session.Close(); journal.Close() })
	f.s.executionHosts = execution.NewHosts()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = f.s.executionHosts.RegisterApproved(ctx, hostConfig, endpoint); err != nil {
		t.Fatal(err)
	}
	ref := execution.Reference{Host: b.Host, Session: b.Session, Grant: b.Grant, Revision: b.Revision}
	resource, err := f.s.executionHosts.Resolve(ctx, b.Actor, b.Workspace, ref)
	if err != nil {
		t.Fatal(err)
	}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 32768
	cfg.Execution = &ref
	cfg.ExecutionWorkspace = &resource
	tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
	decisions := 0
	runner := agents.NewRunner(f.s.agentManager, tools, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		decisions++
		message := api.ChatMessage{Role: "assistant", Content: "Observed the fixture exit."}
		var name string
		var args []byte
		if decisions == 1 {
			name = "workspace_process_start"
			args, _ = json.Marshal(execution.ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessAccountingFixtureHelper$", "--", "offgrid-process-accounting"}, TimeoutSeconds: 10})
		} else {
			var handle string
			state := "running"
			for _, step := range task.Steps {
				if (step.ToolName == "workspace_process_start" || step.ToolName == "workspace_process_wait") && step.Type == "action" {
					var result struct {
						Receipt struct {
							Result execution.ProcessResult `json:"result"`
						} `json:"receipt"`
					}
					if json.Unmarshal([]byte(step.ToolResult), &result) == nil {
						handle = result.Receipt.Result.Handle
						state = result.Receipt.Result.State
					}
				}
			}
			if handle == "" {
				t.Error("missing recorded process handle")
			}
			if decisions == 2 || state == "running" {
				name = "workspace_process_wait"
				args, _ = json.Marshal(map[string]any{"handle": handle, "wait_ms": 1000})
			}
		}
		if name != "" {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{ID: fmt.Sprintf("process-%d", decisions), Type: "function", Function: api.FunctionCall{Name: name, Arguments: string(args)}}}
		}
		finish := "stop"
		if name != "" {
			finish = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
	})
	f.s.agentRunner = runner
	task, err := runner.Create("Run the disposable fixture and inspect its exit", "fixture", b.Actor, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Continue(ctx, task.ID, b.Actor, "start", "", false)
	if err != nil || result.Status != agents.TaskWaiting || result.PendingApproval == nil {
		t.Fatal("custom command lacked exact approval", result, err)
	}
	result, err = runner.Continue(ctx, task.ID, b.Actor, "approve", result.PendingApproval.ID, false)
	if err != nil || result.Status != agents.TaskCompleted {
		t.Fatal(result, err)
	}
	budget := result.Checkpoint.ProcessBudget
	if budget == nil || budget.ReservedStarts != 1 || budget.UnsettledStarts != 0 || budget.MeasuredNS <= 0 || budget.ChargedNS != budget.MeasuredNS || budget.ChargedNS >= int64(10*time.Second) {
		t.Fatal("host exit did not settle root budget", budget)
	}
	saved, _ := f.s.agentManager.GetTask(task.ID)
	if saved.Checkpoint.ProcessBudget.ChargedNS != budget.ChargedNS {
		t.Fatal("in-memory refund was not durable")
	}
}

func TestProcessAccountingFixtureHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "offgrid-process-accounting" {
		t.Skip("subprocess fixture only")
	}
	fmt.Println("disposable process completed")
	os.Exit(0)
}
