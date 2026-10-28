package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestInputWriteAuthorizationRejectsInvalidSelectionBeforeIntent(t *testing.T) {
	task := &agents.Task{ID: "source-before-effects", Prompt: "<a>one</a> <a>two</a>", Config: agents.AgentConfig{InputWriteVersion: 1}}
	for _, args := range []string{
		`{"path":"x","input":"request","after":"<a>","before":"</a>"}`,
		`{"path":"x","input":"correction:foreign-id","after":"<a>","before":"</a>"}`,
		`{"path":"x","input":"request","after":"missing","before":"</a>"}`,
	} {
		var rejected *agents.ToolAuthorizationError
		err := authorizeWorkspaceInvocation(task, execution.InputWriteTool, []byte(args), agents.ToolExecution{})
		if !errors.As(err, &rejected) || rejected.Code != "execution_input_selection_invalid" || task.Checkpoint != nil || len(task.Steps) != 0 {
			t.Fatal("invalid selection crossed pre-dispatch boundary", err)
		}
	}
	task.Prompt = "<a>one</a>"
	if err := authorizeWorkspaceInvocation(task, execution.InputWriteTool, []byte(`{"path":"x","input":"request","after":"<a>","before":"</a>"}`), agents.ToolExecution{}); err != nil {
		t.Fatal("valid scoped selection rejected", err)
	}
}

func TestCanonicalInputWriteUsesOriginalBytesAndIndependentReadback(t *testing.T) {
	testCanonicalInputWrite(t, false)
}

func TestCanonicalInputWriteRepairsReversedMarkersBeforeAnyWrite(t *testing.T) {
	testCanonicalInputWrite(t, true)
}

func testCanonicalInputWrite(t *testing.T, reversedFirst bool) {
	t.Helper()
	f := newConversationFixture(t)
	ref, root := approvedExecutionFixture(t, f.s)
	wanted := "OffGrid test — café 日本語 12345\r\n1| literal"
	if err := os.WriteFile(filepath.Join(root, "untouched.txt"), []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
	decisions := 0
	f.s.agentRunner = agents.NewRunner(f.s.agentManager, tools, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		decisions++
		if task.Config.InputWriteVersion != execution.InputWriteVersion {
			t.Fatal("source contract not pinned")
		}
		found := false
		for _, tool := range available {
			if tool.Function.Name == execution.InputWriteTool {
				found = true
			}
		}
		if !found {
			t.Fatal("authorized source write not offered")
		}
		if reversedFirst && decisions == 1 {
			call := api.ToolCall{ID: "reversed-input", Type: "function", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: `{"path":"result.txt","input":"request","after":"</content>","before":"<content>"}`}}
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}}}}, nil
		}
		if len(task.Steps) == 0 || reversedFirst && decisions == 2 {
			if _, err := os.Stat(filepath.Join(root, "result.txt")); !os.IsNotExist(err) {
				t.Fatal("write happened before valid selection")
			}
			call := api.ToolCall{ID: "write-input", Type: "function", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: `{"path":"result.txt","input":"request","after":"<content>","before":"</content>"}`}}
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}}}}, nil
		}
		if err := agents.ValidateInputWrites(task); err != nil {
			t.Fatal("source evidence not verified", err)
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Selected input written and independently verified."}}}}, nil
	})
	f.s.agentRunner.Observer = f.s.projectConversationTask
	t.Cleanup(func() { f.s.agentRunner.Shutdown(context.Background()) })
	f.request.Content = "Create result.txt exactly: <content>" + wanted + "</content>. Leave other files unchanged."
	f.request.Execution = &ref
	if w := f.submit(f.request); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved := f.waitComplete(t)
	actual, err := os.ReadFile(filepath.Join(root, "result.txt"))
	if err != nil || string(actual) != wanted {
		t.Fatal(string(actual), err)
	}
	original, _ := os.ReadFile(filepath.Join(root, "untouched.txt"))
	if string(original) != "unchanged" {
		t.Fatal("unrelated file changed")
	}
	task, _ := f.s.agentManager.GetTask(saved.Turn.JobID)
	writtenStep := len(task.Steps) - 1
	if writtenStep != map[bool]int{false: 0, true: 1}[reversedFirst] || !strings.Contains(task.Steps[writtenStep].ToolResult, `"source_input"`) {
		t.Fatal(task.Steps)
	}
	var view map[string]any
	json.Unmarshal([]byte(task.Steps[writtenStep].ToolResult), &view)
	view["source_input"].(map[string]any)["content_digest"] = strings.Repeat("0", 64)
	bad, _ := json.Marshal(view)
	task.Steps[writtenStep].ToolResult = string(bad)
	if tools.ValidateExecutionCompletion(task) == nil {
		t.Fatal("tampered source accepted as completed")
	}
}

func TestInputWriteCannotExpandOldJobsOrReadOnlyGrants(t *testing.T) {
	f := newConversationFixture(t)
	ref, _ := approvedExecutionFixture(t, f.s)
	resource, err := f.s.executionHosts.Resolve(context.Background(), "local-admin", f.s.workspaceID, ref)
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.Execution = &ref
	config.ExecutionWorkspace = &resource
	task := &agents.Task{ID: "source-scope", Actor: "local-admin", Status: agents.TaskRunning, Prompt: "<a>private</a>", Config: config}
	tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
	d, _ := executionDescriptor(execution.InputWriteTool)
	e := agents.ToolExecution{Actor: task.Actor, RunID: task.ID, ExpectedCapability: &d}
	if tools.executionAccess(context.Background(), task, execution.InputWriteTool, e) == nil {
		t.Fatal("old unprofiled job gained source access")
	}
	task.Config.InputWriteVersion = 1
	task.Config.ExecutionPolicy = &execution.Permissions{Read: true}
	if tools.executionAccess(context.Background(), task, execution.InputWriteTool, e) == nil {
		t.Fatal("read grant became a write grant")
	}
	for _, tool := range tools.workspaceTools(context.Background(), task) {
		if tool.Function.Name == execution.InputWriteTool {
			t.Fatal("read-only discovery leaked source write")
		}
	}
	if _, err := os.Stat(filepath.Join(resource.Root, "result.txt")); !os.IsNotExist(err) {
		t.Fatal("scope check caused host effects")
	}
}
