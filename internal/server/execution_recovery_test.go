package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type executionLostResultFixture struct {
	*browserRunTools
	calls        int
	afterReceipt bool
}

func (f *executionLostResultFixture) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, e agents.ToolExecution) (string, error) {
	f.calls++
	if f.afterReceipt {
		if _, err := f.browserRunTools.ExecuteWithPolicy(ctx, name, args, e); err != nil {
			return "", err
		}
		return "", errors.New("fixture lost service result")
	}
	if err := f.Authorize(ctx, name, args, e); err != nil {
		return "", err
	}
	task, _ := f.server.agentManager.GetTask(e.RunID)
	var values struct{ Path, Content string }
	if json.Unmarshal(args, &values) != nil {
		return "", errors.New("fixture args")
	}
	providerArgs, _ := json.Marshal(map[string]any{"path": values.Path, "data": []byte(values.Content)})
	ref := *task.Config.Execution
	b := execution.Binding{Actor: task.Actor, Workspace: f.server.workspaceID, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: task.ID}
	intent, err := execution.NewIntent(b, e.CallID, "file_write", providerArgs, false)
	if err != nil {
		return "", err
	}
	if err = f.server.agentManager.RecordExecutionIntent(task.ID, task.Actor, name, args, intent); err != nil {
		return "", err
	}
	if err = f.server.executionHosts.Admit(ctx, task.Actor, b.Workspace, task.ID, ref); err != nil {
		return "", err
	}
	response, err := f.server.executionHosts.Call(ctx, task.Actor, b.Workspace, task.ID, e.CallID, ref, "file_write", providerArgs)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		return "", errors.New("fixture dispatch failed")
	}
	return "", errors.New("fixture dropped result before service receipt")
}

func TestWorkspaceRecoveryReadsNativeJournalAndNeverReplaysOrClaimsCompletion(t *testing.T) {
	for _, afterReceipt := range []bool{false, true} {
		s := profileFixture(t)
		ref, root := approvedExecutionFixture(t, s)
		tools := &executionLostResultFixture{browserRunTools: &browserRunTools{RunTools: s.toolRegistry, server: s}, afterReceipt: afterReceipt}
		runner := agents.NewRunner(s.agentManager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "workspace_write", Arguments: `{"path":"once.txt","content":"café 日本語"}`}}}}}}}, nil
		})
		s.agentRunner = runner
		config := agents.DefaultAgentConfig()
		config.TaskFirst = true
		config.ContextWindow = 32768
		resource, err := s.executionHosts.Resolve(context.Background(), "local-admin", s.workspaceID, ref)
		if err != nil {
			t.Fatal(err)
		}
		config.Execution = &ref
		config.ExecutionWorkspace = &resource
		task, err := runner.Create("write exact Unicode", "model", "local-admin", config)
		if err != nil {
			t.Fatal(err)
		}
		task, err = runner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
		if err == nil || task == nil || tools.calls != 1 {
			t.Fatal(task, err, tools.calls)
		}
		if afterReceipt {
			if task.Status != agents.TaskInterrupted || task.ErrorCode != "execution_result_recovered" {
				t.Fatal(task)
			}
		} else {
			if task.Status != agents.TaskUncertain || !agents.CanRecoverExecution(task) {
				t.Fatal(task)
			}
			// Recovery may return the original receipt, not overwrite this later edit.
			if err = os.WriteFile(filepath.Join(root, "once.txt"), []byte("later manual edit"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{`null`, `{"result":"done"}`, `{} {}`} {
				w := httptest.NewRecorder()
				request := httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/recover-execution", strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				s.handleJobRead(w, request)
				if w.Code != 400 {
					t.Fatal("replacement recovery input accepted", w.Code, w.Body.String())
				}
			}
			w := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/recover-execution", strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			s.handleJobRead(w, request)
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			task, _ = s.agentManager.GetTask(task.ID)
			if content, _ := os.ReadFile(filepath.Join(root, "once.txt")); string(content) != "later manual edit" {
				t.Fatal("original mutation replayed")
			}
		}
		if task.Status != agents.TaskInterrupted || task.Result != "" || len(task.Steps) != 1 || task.Steps[0].Provenance != "execution_receipt" || tools.calls != 1 {
			t.Fatal(task, tools.calls)
		}
		if taskResponse(task)["can_recover_execution"] != false {
			t.Fatal("consumed receipt offered twice")
		}
	}
}
