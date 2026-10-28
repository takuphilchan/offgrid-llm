package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnDemandWorkspaceAttachesToCanonicalConversationWithoutReplay(t *testing.T) {
	f := newConversationFixture(t)
	ref, root := approvedExecutionFixture(t, f.s)
	f.s.config.EnableExecutionPreview = true
	if err := os.WriteFile(filepath.Join(root, "fixture.txt"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
	runner := agents.NewRunner(f.s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		name, args := execution.AccessRequestTool, `{"access":"read"}`
		if task.Config.Execution != nil {
			name, args = "workspace_list", `{}`
			for _, step := range task.Steps {
				if step.Type == "action" {
					return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Observed fixture.txt."}}}}, nil
				}
			}
		} else {
			for _, tool := range available {
				if serviceFileTool(tool.Function.Name) || strings.HasPrefix(tool.Function.Name, "workspace_") {
					t.Error("ungranted file capability", tool.Function.Name)
				}
			}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "fixture-call-" + name, Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	})
	runner.Observer = f.s.projectConversationTask
	f.s.agentRunner = runner
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	f.request.Content = "List the selected local folder"
	f.request.ExecutionRequestVersion = 1
	response := f.submit(f.request)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var waiting *agents.Task
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tasks := f.s.agentManager.ListTasks()
		if len(tasks) == 1 && tasks[0].Status == agents.TaskInput {
			waiting = tasks[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if waiting == nil || waiting.PendingInput.Kind != "workspace" || waiting.Config.Execution != nil {
		t.Fatal("did not pause for local authority", waiting)
	}
	attach := func(reference execution.Reference) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"input_id": waiting.PendingInput.ID, "execution": reference})
		w := httptest.NewRecorder()
		f.s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+waiting.ID+"/input", strings.NewReader(string(body))), waiting.ID)
		return w
	}
	foreign := ref
	foreign.Host = "another-host"
	if w := attach(foreign); w.Code != 409 {
		t.Fatal("foreign reference accepted", w.Code, w.Body.String())
	}
	if w := attach(ref); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved := f.waitComplete(t)
	if saved.Turn.JobID != waiting.ID || saved.Turn.Output != "Observed fixture.txt." {
		t.Fatal("canonical identity changed", saved.Turn)
	}
	if w := attach(ref); w.Code != 202 {
		t.Fatal("idempotent attachment failed", w.Code, w.Body.String())
	}
	task, _ := f.s.agentManager.GetTask(waiting.ID)
	actions, attached := 0, 0
	for _, step := range task.Steps {
		if step.Type == "action" {
			actions++
		}
		if step.Type == "input_resolved" {
			attached++
		}
	}
	if actions != 1 || attached != 1 || len(f.s.agentManager.ListTasks()) != 1 {
		t.Fatal("replayed attachment or operation", actions, attached)
	}
	content, _ := os.ReadFile(filepath.Join(root, "fixture.txt"))
	if string(content) != "untouched" {
		t.Fatal("read changed files")
	}
}

func TestWorkspaceRequestNeverGrantsAuthorityAndEnforcesCeilings(t *testing.T) {
	d := execution.AccessRequestDescriptor()
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ExecutionRequestVersion = 1
	cfg.ExecutionPolicy = &execution.Permissions{Read: true}
	task := &agents.Task{ID: "run", Actor: "alice", Config: cfg}
	e := agents.ToolExecution{RunID: "run", Actor: "alice", ExpectedCapability: &d}
	for _, raw := range []string{`{"access":"edit"}`, `{"access":"trusted"}`, `{"access":"read","folder":"/user"}`, `{"access":"read","access":"edit"}`, `{"access":"shell"}`, `{}`} {
		if err := workspaceAccessRequest(task, json.RawMessage(raw), e); !errors.Is(err, capabilities.ErrDenied) {
			t.Fatal("invalid request accepted", raw, err)
		}
	}
	var input *agents.InputRequired
	if err := workspaceAccessRequest(task, json.RawMessage(`{"access":"read"}`), e); !errors.As(err, &input) || input.Request.Kind != "workspace" || task.Config.Execution != nil {
		t.Fatal("request created authority", err)
	}
	task.ParentID = "parent"
	if err := workspaceAccessRequest(task, json.RawMessage(`{"access":"read"}`), e); !errors.Is(err, capabilities.ErrDenied) {
		t.Fatal("child requested authority", err)
	}
}
