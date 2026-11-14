package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerQuestionHTTPAnswerIsDurableWithoutDispatch(t *testing.T) {
	s := profileFixture(t)
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		name, args := "delegate_tasks", `{"tasks":[{"key":"review","goal":"Review compatibility","depends_on":[],"tools":[]}]}`
		if task.Worker != nil {
			name, args = "worker_question", `{"question":"Which compatibility version should I check?"}`
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	c, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.TaskFirst, config.ScopedCapabilities, config.WorkerQuestionsVersion = true, true, 1
	config.ContextWindow = 16384
	receipt, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: c.TargetID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Review compatibility"}}}, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.agentRunner.Continue(context.Background(), receipt.Work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatal(root, err)
	}
	child, err := s.agentRunner.Continue(context.Background(), root.ChildHistory[0].ID, scope.Actor, "start", "", false)
	if err != nil || child.WorkerQuestion == nil {
		t.Fatal(child, err)
	}
	path := "/api/v2/workers/" + child.Worker.SessionID
	do := func(method, route string, payload any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		s.handleWorkerSessions(w, httptest.NewRequest(method, route, strings.NewReader(string(data))))
		return w
	}
	w := do("GET", path, nil)
	var worker api.WorkerSession
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &worker) != nil || worker.Question == nil {
		t.Fatal(w.Code, w.Body.String())
	}
	command := api.WorkerCommand{RequestID: "answer", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: worker.Question.ID, Content: "Version 2"}}
	s.registry = nil // Answer acceptance must not depend on available inference.
	w = do("POST", path+"/commands", command)
	if w.Code != 202 || !strings.Contains(w.Body.String(), `"state":"accepted"`) || !strings.Contains(w.Body.String(), `"state":"answered"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = do("POST", path+"/commands", command); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if calls != 2 || len(s.agentManager.ListTasks()) != 2 {
		t.Fatal("answer dispatched or recreated work", calls)
	}
	ready, _ := s.agentManager.GetTask(child.ID)
	if ready.Status != agents.TaskPending || ready.Config.MaxIterations != child.Config.MaxIterations || ready.Checkpoint.Iteration != child.Checkpoint.Iteration {
		t.Fatal("answer reset allowance", ready)
	}
}
