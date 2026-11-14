package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerHTTPActorIsolationAndControlWithoutModel(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, key, err := s.userStore.CreateUser("alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, foreign, err := s.userStore.CreateUser("bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		call := api.ToolCall{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"review","goal":"Review the question","depends_on":[],"tools":[]}]}`}}
		message := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: message}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: alice.ID}
	conversation, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.ScopedCapabilities = true
	accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: conversation.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Review this"}}}, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.agentRunner.Continue(context.Background(), accepted.Work.RunID, alice.ID, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatal(root, err)
	}
	child, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
	worker, err := s.agentManager.WorkerSession(scope, child.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	// Clients discover the retained worker through the ordinary parent snapshot;
	// no profile selection, second executor or client-created session is needed.
	data, err := json.Marshal(taskResponse(root))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Children []struct {
			ID       string `json:"id"`
			WorkerID string `json:"worker_id"`
		} `json:"children"`
	}
	if err = json.Unmarshal(data, &snapshot); err != nil || len(snapshot.Children) != 1 || snapshot.Children[0].ID != child.ID || snapshot.Children[0].WorkerID != worker.ID {
		t.Fatal("worker session absent from parent snapshot", string(data), err)
	}
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleWorkerSessions))
	do := func(token, method, path string, payload any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/api/v2/workers/" + worker.ID
	if w := do(key, "GET", path, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := do(foreign, "GET", path, nil); w.Code != 404 {
		t.Fatal("foreign worker exposed", w.Code, w.Body.String())
	}
	message := api.WorkerCommand{RequestID: "message", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Keep citations"}}
	w := do(key, "POST", path+"/commands", message)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt api.ConversationCommandReceipt
	if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if w := do(key, "GET", path+"/messages?limit=1", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"accepted"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := do(key, "POST", path+"/commands", message); w.Code != 200 {
		t.Fatal("retry did not return recorded command", w.Code, w.Body.String())
	}
	// Preview admission and model availability are unrelated to accepting Stop.
	s.config.EnableConversationPreview = false
	s.registry = nil
	stop := api.WorkerCommand{RequestID: "stop", TargetID: worker.ID, ExpectedRevision: receipt.Revision, Kind: "stop"}
	if w := do(foreign, "POST", path+"/commands", stop); w.Code != http.StatusNotFound {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := do(key, "POST", path+"/commands", stop); w.Code != 202 || !strings.Contains(w.Body.String(), `"stopped":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
