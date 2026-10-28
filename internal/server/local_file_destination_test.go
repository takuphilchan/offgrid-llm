package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Real service/host/storage, with synthetic model choices. Real Qwen/Meta-Llama
// runs are separately required; this pins the destination and access boundary.
func TestLocalFileCreationAfterAccessPreservesExactExtensionlessName(t *testing.T) {
	f := newConversationFixture(t)
	ref, root := approvedExecutionFixture(t, f.s)
	f.s.config.EnableExecutionPreview = true
	// Match the interactive service's allocation; the generic conversation
	// fixture defaults to a smaller text-only context.
	f.s.config.MaxContextSize = 16384
	f.s.config.ChatContextSize = 8192
	f.request.Profile = "interactive"
	f.request.MaxTokens = 1024
	if err := os.WriteFile(filepath.Join(root, "neighbor"), []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	r := agents.NewRunner(f.s.agentManager, &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		for _, tool := range tools {
			if tool.Function.Name == taskArtifactToolName || serviceFileTool(tool.Function.Name) {
				t.Error("wrong destination offered", tool.Function.Name)
			}
		}
		name, args := execution.AccessRequestTool, `{"access":"edit"}`
		if task.Config.Execution != nil {
			name, args = "workspace_write", `{"path":"zemo","content":"hello there"}`
			for _, step := range task.Steps {
				if step.Type == "action" {
					return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Created zemo in the approved folder."}}}}, nil
				}
			}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "model-call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	})
	f.s.agentRunner = r
	r.Observer = f.s.projectConversationTask
	t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
	f.request.Content = "create file in this directory named zemo and the files needs to say hello there"
	f.request.ExecutionRequestVersion = 1
	response := f.submit(f.request)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var waiting *agents.Task
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, task := range f.s.agentManager.ListTasks() {
			if task.Status == agents.TaskInput {
				waiting = task
				break
			}
		}
		if waiting != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if waiting == nil || waiting.PendingInput.Kind != "workspace" || waiting.PendingInput.Mode != "edit" {
		t.Fatal("missing edit request", waiting)
	}
	if _, err := os.Stat(filepath.Join(root, "zemo")); !os.IsNotExist(err) {
		t.Fatal("write before consent", err)
	}
	body, _ := json.Marshal(map[string]any{"input_id": waiting.PendingInput.ID, "execution": ref})
	w := httptest.NewRecorder()
	f.s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+waiting.ID+"/input", strings.NewReader(string(body))), waiting.ID)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitComplete(t)
	task, _ := f.s.agentManager.GetTask(waiting.ID)
	writes := 0
	for _, step := range task.Steps {
		if step.Type == "action" {
			writes++
			if step.ToolName != "workspace_write" || !strings.Contains(step.ToolResult, `"verified":true`) {
				t.Fatal("wrong write receipt", step)
			}
		}
	}
	if writes != 1 {
		t.Fatal("duplicate write", writes)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatal("unexpected outputs", entries, err)
	}
	for name, want := range map[string]string{"zemo": "hello there", "neighbor": "untouched"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != want {
			t.Fatal(name, string(data), err)
		}
	}
}
