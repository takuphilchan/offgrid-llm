package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkspaceRejectedReadAllowsBoundedCorrectionWithoutMutation(t *testing.T) {
	for _, repeats := range []bool{false, true} {
		t.Run(map[bool]string{false: "corrected", true: "bounded"}[repeats], func(t *testing.T) {
			f := newConversationFixture(t)
			ref, root := approvedExecutionFixture(t, f.s)
			if err := os.Mkdir(filepath.Join(root, "reports"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "untouched.txt"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			tools := &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}
			f.s.agentRunner = agents.NewRunner(f.s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				name, args := "workspace_read", `{"path":"reports"}`
				if !repeats && calls == 2 {
					if !strings.Contains(messages[len(messages)-1].StringContent(), `"effects":"none"`) {
						t.Error("known read failure missing")
					}
					name, args = "workspace_list", `{"path":"reports"}`
				}
				message := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "read-" + string(rune('a'+calls)), Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}
				reason := "tool_calls"
				if !repeats && calls == 3 {
					message = api.ChatMessage{Role: "assistant", Content: "The observed reports directory is empty."}
					reason = "stop"
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}, nil
			})
			f.s.agentRunner.Observer = f.s.projectConversationTask
			t.Cleanup(func() { f.s.agentRunner.Shutdown(context.Background()) })
			f.request.Content, f.request.Execution = "Inspect the reports directory without editing anything.", &ref
			if w := f.submit(f.request); w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			if !repeats {
				f.waitComplete(t)
			} else {
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					current := f.s.agentManager.ListTasks()
					if len(current) == 1 && current[0].Status == agents.TaskInterrupted {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			jobs := f.s.agentManager.ListTasks()
			if len(jobs) != 1 {
				t.Fatal("new root created", len(jobs))
			}
			saved := jobs[0]
			if repeats {
				if saved.Status != agents.TaskInterrupted || saved.ErrorCode != "workspace_read_repair_limit" || calls != 4 || saved.Checkpoint.WorkspaceReadFailures != 4 {
					t.Fatal(saved.Status, saved.ErrorCode, calls)
				}
			} else if saved.Status != agents.TaskCompleted || calls != 3 || saved.Checkpoint.WorkspaceReadFailures != 1 {
				t.Fatal(saved.Status, saved.Error, calls)
			}
			data, err := os.ReadFile(filepath.Join(root, "untouched.txt"))
			if err != nil || string(data) != "original" || saved.Checkpoint.ExecutingCall != "" {
				t.Fatal("read repair changed files or left dispatch uncertain", err)
			}
		})
	}
}
