package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRevokedFocusedWorkAllowsIndependentConversationWithoutDerivedContext(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.EnableConversationPreview, s.config.DefaultModel = true, "model"
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "An independent explanation"}}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	config := agents.DefaultAgentConfig()
	config.KnowledgeDependencies = []string{"revoked-source"}
	created, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "source-work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Compare private sources", Acceptance: []string{"Private derived criterion"}}}}, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	s.agentRunner.SourceAccess = s.taskSourceAllowed
	c, _ = s.agentManager.Conversation(scope, c.ID)
	command := api.ConversationInputCommand{RequestID: "side-question", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Explain the general process"}}
	w := controlHTTP(s, "POST", "/api/v2/conversations/"+c.ID+"/inputs", command)
	var receipt api.ConversationCommandReceipt
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	foreground, err := s.prepareConversationTurn(context.Background(), scope, c.ID, receipt.Input.ID)
	if err != nil {
		t.Fatal(err)
	}
	contextText, _ := json.Marshal(foreground.Config.ConversationContext)
	if strings.Contains(string(contextText), "Private derived criterion") || len(foreground.Config.KnowledgeDependencies) != 0 || !strings.Contains(string(contextText), "context_unavailable") {
		t.Fatal("source context leaked or blocked conversation", string(contextText))
	}
	if _, err = s.agentRunner.Continue(context.Background(), foreground.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.agentRunner.ApplyConversationDecision(scope, c.ID, receipt.Input.ID); err != nil {
		t.Fatal("side answer could not settle", err)
	}
	root, _ := s.agentManager.GetTask(created.Work.RunID)
	if root.Checkpoint.BudgetedModelCalls != 0 || root.InstructionVersion != 1 {
		t.Fatal("side answer spent or changed revoked work")
	}
	c, _ = s.agentManager.Conversation(scope, c.ID)
	command.RequestID, command.ExpectedRevision, command.Payload.Intent = "blocked-correction", c.Revision, "correction"
	if w = controlHTTP(s, "POST", "/api/v2/conversations/"+c.ID+"/inputs", command); w.Code != 403 {
		t.Fatal("correction bypassed revoked work", w.Code, w.Body.String())
	}
}

func TestConversationInputRevocationRedactsReadsEventsAndStopReceipts(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		message := api.ChatMessage{Role: "assistant", Content: "Private derived reply"}
		finish := "stop"
		if len(task.Steps) == 0 {
			finish, message.Content = "tool_calls", ""
			message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "conversation_decide", Arguments: `{"kind":"new_work","instruction_version":0,"expected_result":"Private derived acceptance"}`}}}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: finish, Message: message}}}, nil
	})
	input := acceptContextInput(t, s, scope, c.ID, "input", "Keep my original request")
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.KnowledgeDependencies = []string{"revoked-source"}
	completeContextInput(t, s, scope, input, config)
	path := "/api/v2/conversations/" + c.ID
	for _, suffix := range []string{"/inputs", "/inputs/" + input.ID, "/events?limit=200"} {
		response := controlHTTP(s, "GET", path+suffix, nil)
		body := response.Body.String()
		if response.Code != 200 || strings.Contains(body, "Private derived") || !strings.Contains(body, "Keep my original request") || !strings.Contains(body, `"context_unavailable":true`) {
			t.Fatal("derived content escaped revocation", suffix, response.Code, body)
		}
	}
	c, _ = s.agentManager.Conversation(scope, c.ID)
	command := api.ConversationInputControlCommand{RequestID: "stop", TargetID: input.ID, ExpectedRevision: c.Revision, Kind: "stop"}
	for _, want := range []int{202, 200} {
		response := controlHTTP(s, "POST", path+"/inputs/"+input.ID+"/commands", command)
		if response.Code != want || strings.Contains(response.Body.String(), "Private derived") || !strings.Contains(response.Body.String(), `"context_unavailable":true`) {
			t.Fatal("Stop blocked or leaked content", response.Code, response.Body.String())
		}
	}
	stored, _ := s.agentManager.ConversationInput(scope, c.ID, input.ID)
	if stored.Decision == nil || stored.Decision.ExpectedResult != "Private derived acceptance" || stored.ContextUnavailable {
		t.Fatal("public redaction changed canonical evidence", stored)
	}
}
