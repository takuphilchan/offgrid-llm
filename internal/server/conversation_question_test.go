package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationQuestionHTTPAnswerWithoutModelAdmission(t *testing.T) {
	s := profileFixture(t)
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		name, args := "delegate_tasks", `{"tasks":[{"key":"review","goal":"Compare runtimes","depends_on":[],"tools":[]}]}`
		if task.Worker != nil {
			name, args = "worker_question", `{"question":"Include offline runtimes?"}`
		} else if task.Delegation != nil && task.Delegation.QuestionReview != nil {
			name, args = "worker_answer", `{"content":"Should the comparison include offline runtimes?","needs_user":true}`
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
	accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: c.TargetID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Compare runtimes"}}}, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.Continue(context.Background(), accepted.Work.RunID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.agentRunner.StartCoordinator(ctx)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		root, _ := s.agentManager.GetTask(accepted.Work.RunID)
		if root.Status == agents.TaskInput && root.PendingInput != nil && root.PendingInput.Kind == "worker_clarification" {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("primary did not ask a question")
		case <-ticker.C:
		}
	}
	cancel()
	path := "/api/v2/conversations/" + c.TargetID
	response := controlHTTP(s, "GET", path, nil)
	var current api.Conversation
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.PendingQuestion == nil {
		t.Fatal(response.Code, response.Body.String())
	}
	if calls.Load() != 3 {
		t.Fatal("unexpected classification or worker calls", calls.Load())
	}
	var discussion agents.AgentConfig
	if err = s.appendFocusedWorkContext(scope, current.ID, &api.ConversationInboxEntry{WorkID: accepted.Work.ID}, &discussion); err != nil || len(discussion.ConversationContext) != 1 || !strings.Contains(discussion.ConversationContext[0].StringContent(), current.PendingQuestion.Content) {
		t.Fatal("foreground discussion omitted its pending clarification", discussion.ConversationContext, err)
	}
	s.config.EnableConversationPreview = false
	s.registry = nil
	command := api.ConversationInputCommand{RequestID: "answer", TargetID: current.ID, ExpectedRevision: current.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Yes, include offline runtimes.", Intent: "answer", QuestionID: current.PendingQuestion.ID, Origin: "dictation"}}
	response = controlHTTP(s, "POST", path+"/inputs", command)
	var receipt api.ConversationCommandReceipt
	if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &receipt) != nil || receipt.Input == nil || receipt.Input.Incorporation != "worker_answer" {
		t.Fatal(response.Code, response.Body.String())
	}
	if response = controlHTTP(s, "POST", path+"/inputs", command); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if calls.Load() != 3 || len(s.agentManager.ListTasks()) != 2 {
		t.Fatal("answer dispatched inference or recreated work")
	}
	response = controlHTTP(s, "GET", path, nil)
	current = api.Conversation{}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.PendingQuestion != nil {
		t.Fatal("read revived question", response.Body.String())
	}
	var projection sessions.Session
	request := httptest.NewRequest("GET", "/", nil)
	if err = s.projectConversationInput(request, *receipt.Input, &projection); err != nil {
		t.Fatal(err)
	}
	if len(projection.Messages) != 3 || projection.Messages[0].Content != "Should the comparison include offline runtimes?" || projection.Messages[1].Content != command.Payload.Content {
		t.Fatal("lost question/answer context", projection.Messages)
	}
	var assembled agents.AgentConfig
	if err = s.appendConversationInputContext(scope, current.ID, *receipt.Input, &assembled); err != nil {
		t.Fatal(err)
	}
	for _, message := range assembled.ConversationContext {
		if strings.Contains(message.StringContent(), "queued and has no execution attempt") {
			t.Fatal("accepted answer reported as queued")
		}
	}
	// Exercise the real work envelope with the same Go client used by CLI
	// controls, rather than a fixture that accidentally returns a bare object.
	service := httptest.NewServer(http.HandlerFunc(s.handleControlWork))
	defer service.Close()
	client, err := serviceclient.New(service.URL, "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	work, err := client.WorkItem(context.Background(), accepted.Work.ID)
	if err != nil || work.ID != accepted.Work.ID || work.RunID != accepted.Work.RunID {
		t.Fatal(work, err)
	}
	if _, err = client.WorkCommand(context.Background(), api.WorkCommand{RequestID: "cross-client-stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"}); err != nil {
		t.Fatal(err)
	}
	root, _ := s.agentManager.GetTask(work.RunID)
	if root.Status != agents.TaskCancelled {
		t.Fatal("Go client did not stop the same work", root.Status)
	}
	// Enabling authenticated actors revokes this fixture's local-admin source
	// authority. Historical user text survives; derived question prose does not.
	s.config.RequireAuth = true
	redacted := s.publicConversationInput(request, *receipt.Input)
	if redacted.Question != nil || !redacted.ContextUnavailable || redacted.Input.Content != command.Payload.Content {
		t.Fatal("revoked question escaped public projection", redacted)
	}
	var restricted agents.AgentConfig
	if err = s.appendConversationInputContext(scope, current.ID, *receipt.Input, &restricted); err != nil {
		t.Fatal(err)
	}
	for _, message := range restricted.ConversationContext {
		if strings.Contains(message.StringContent(), "Should the comparison include offline runtimes?") {
			t.Fatal("revoked question entered model context")
		}
	}
}
