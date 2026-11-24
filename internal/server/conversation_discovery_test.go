package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type metadataHost struct {
	config execution.Context
	calls  atomic.Int32
}

func (h *metadataHost) Check(context.Context, execution.Binding) (execution.Context, error) {
	h.calls.Add(1)
	return h.config, nil
}
func (h *metadataHost) Admit(context.Context, execution.Binding) error {
	h.calls.Add(1)
	return errors.New("unexpected host admission")
}
func (h *metadataHost) Call(context.Context, execution.Request) (execution.Response, error) {
	h.calls.Add(1)
	return execution.Response{}, errors.New("unexpected host call")
}
func (h *metadataHost) Cancel(context.Context, execution.Binding) error { h.calls.Add(1); return nil }
func (h *metadataHost) Close() error                                    { return nil }

func TestConversationMetadataDoesNotContactSelectedHost(t *testing.T) {
	s, scope, conversation := contextInboxFixture(t)
	s.config.EnableExecutionPreview = true
	s.executionHosts = execution.NewHosts()
	host := &metadataHost{config: execution.Context{Version: 1, Binding: execution.Binding{Actor: scope.Actor, Workspace: scope.Workspace, Host: "host", Session: "session", Grant: "grant", Revision: 1, Job: "supervisor"}, Backend: "local", Root: t.TempDir(), Directory: ".", Permissions: execution.Permissions{Read: true}, ProjectHostID: strings.Repeat("a", 64)}}
	if err := s.executionHosts.RegisterApproved(context.Background(), host.config, host); err != nil {
		t.Fatal(err)
	}
	command := agents.ConversationCommand{RequestID: "bind-metadata-host", TargetID: conversation.ID, ExpectedRevision: conversation.Revision, Kind: "use_project_folder", Payload: agents.ConversationCommandPayload{Folder: &api.ExecutionReference{Host: "host", Session: "session", Grant: "grant", Revision: 1}}}
	bound, err := s.agentManager.ApplyProjectFolder(scope, command, execution.ProjectContextSnapshot{Identity: strings.Repeat("a", 64), Name: "metadata fixture"})
	if err != nil {
		t.Fatal(err)
	}
	task := preparedMetadataTurn(t, s, scope, bound.Conversation)
	host.calls.Store(0)
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	names := metadataNames(adapter.ConversationToolMetadata(context.Background(), task))
	if !names["workspace_read"] || !names[execution.AccessRequestTool] || names["read_file"] {
		t.Fatal("selected-folder metadata changed scope", names)
	}
	d, _ := adapter.Capability("workspace_read")
	if _, err := adapter.ExecuteWithPolicy(context.Background(), d.Name, json.RawMessage(`{"path":"private.txt"}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, Approved: true, ExpectedCapability: &d}); err == nil {
		t.Fatal("metadata dispatched host read")
	}
	if host.calls.Load() != 0 {
		t.Fatal("metadata or forged dispatch contacted host", host.calls.Load())
	}
}

type metadataBrokerPolicy struct{ denied atomic.Bool }

func (p *metadataBrokerPolicy) Evaluate(context.Context, capabilities.Request) capabilities.Result {
	if p.denied.Load() {
		return capabilities.Result{Decision: capabilities.Deny}
	}
	return capabilities.Result{Decision: capabilities.RequireApproval}
}

func TestConversationMetadataDoesNotContactMCPOrTransferApproval(t *testing.T) {
	s, scope, conversation := contextInboxFixture(t)
	remote := agents.NewToolRegistry()
	var requests, effects atomic.Int32
	remote.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "docs", Description: "Synthetic documentation", Parameters: map[string]interface{}{"type": "object"}}}, func(context.Context, json.RawMessage) (string, error) { effects.Add(1); return "not called", nil })
	handler := remote.MCPHTTPHandler("metadata-fixture")
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); handler.ServeHTTP(w, r) }))
	defer endpoint.Close()
	if _, err := s.toolRegistry.LoadMCPTools("docs", endpoint.URL); err != nil {
		t.Fatal(err)
	}
	defer s.toolRegistry.RemoveMCPServer("docs")
	policy := &metadataBrokerPolicy{}
	s.toolRegistry.SetCapabilityBroker(capabilities.NewBroker(policy))
	task := preparedMetadataTurn(t, s, scope, conversation)
	requests.Store(0)
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	if !metadataNames(adapter.ConversationToolMetadata(context.Background(), task))["mcp__docs__docs"] {
		t.Fatal("approval-required metadata was hidden")
	}
	d, _ := adapter.Capability("mcp__docs__docs")
	if _, err := adapter.ExecuteWithPolicy(context.Background(), d.Name, json.RawMessage(`{}`), agents.ToolExecution{Actor: task.Actor, RunID: task.ID, Approved: true, ExpectedCapability: &d}); err == nil {
		t.Fatal("metadata transferred approval")
	}
	policy.denied.Store(true)
	if metadataNames(adapter.ConversationToolMetadata(context.Background(), task))[d.Name] {
		t.Fatal("live provider denial ignored")
	}
	if requests.Load() != 0 || effects.Load() != 0 {
		t.Fatal("metadata contacted provider", requests.Load(), effects.Load())
	}
}

func preparedMetadataTurn(t *testing.T, s *Server, scope agents.ControlScope, conversation *agents.Conversation) *agents.Task {
	t.Helper()
	s.config.DefaultModel, s.config.EnableConversationPreview = "model", true
	input := acceptContextInput(t, s, scope, conversation.ID, "discover", "What tools could help with this work?")
	task, err := s.prepareConversationTurn(context.Background(), scope, conversation.ID, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Config.ConversationDiscovery == nil || task.Config.ConversationDiscovery.Version != 1 || len(task.Config.AllowedCapabilities) != 0 || task.Config.Execution != nil || !task.Config.RestrictComputer {
		t.Fatal("metadata pin retained execution authority", task.Config)
	}
	return task
}

func metadataNames(rows []capabilities.Descriptor) map[string]bool {
	names := map[string]bool{}
	for _, row := range rows {
		names[row.Name] = true
	}
	return names
}

func TestConversationMetadataLiveScopeAndIdentity(t *testing.T) {
	s, scope, conversation := contextInboxFixture(t)
	task := preparedMetadataTurn(t, s, scope, conversation)
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	names := metadataNames(adapter.ConversationToolMetadata(context.Background(), task))
	if !names["calculator"] || !names["read_file"] || !names[computerAccessToolName] || names["shell"] {
		t.Fatal("wrong default discovery ceiling", names)
	}
	for _, name := range []string{"calculator", "read_file", computerAccessToolName} {
		d, _ := adapter.Capability(name)
		err := adapter.Authorize(context.Background(), name, json.RawMessage(`{}`), agents.ToolExecution{Actor: scope.Actor, RunID: task.ID, Approved: true, ExpectedCapability: &d})
		if !errors.Is(err, capabilities.ErrDenied) {
			t.Fatal("metadata authorized execution", name, err)
		}
	}
	if err := s.toolRegistry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	s.toolRegistry.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "read_file", Description: "Changed provider identity", Parameters: map[string]interface{}{"type": "object"}}}, func(context.Context, json.RawMessage) (string, error) {
		providerCalls++
		return "must not execute", nil
	})
	names = metadataNames(adapter.ConversationToolMetadata(context.Background(), task))
	if names["calculator"] || names["read_file"] || providerCalls != 0 {
		t.Fatal("disabled or replaced metadata survived", names, providerCalls)
	}
	current, _ := s.agentManager.Conversation(scope, conversation.ID)
	noTools, noComputer := []agents.ProfileToolReference{}, false
	preferences := current.Preferences
	preferences.Restrictions.Tools, preferences.Restrictions.Computer = &noTools, &noComputer
	_, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "tighten", TargetID: current.ID, ExpectedRevision: current.Revision, Kind: "configure_conversation", Payload: agents.ConversationCommandPayload{Preferences: &preferences}})
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.ConversationToolMetadata(context.Background(), task); len(got) != 0 {
		t.Fatal("current explicit empty restrictions ignored", got)
	}
	forged := *task
	forged.Actor = "foreign"
	if len(adapter.ConversationToolMetadata(context.Background(), &forged)) != 0 {
		t.Fatal("foreign actor read metadata")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if len(adapter.ConversationToolMetadata(ctx, task)) != 0 {
		t.Fatal("cancelled discovery returned metadata")
	}
}

func TestConversationMetadataCurrentActorAndProfile(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	user, _, err := s.userStore.CreateUser("metadata-owner", "fixture-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: user.ID}
	receipt, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "metadata-conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	task := preparedMetadataTurn(t, s, scope, receipt.Conversation)
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	if !metadataNames(adapter.ConversationToolMetadata(context.Background(), task))["read_file"] {
		t.Fatal("admin metadata absent")
	}
	if err := s.userStore.UpdateUser(user.ID, map[string]any{"role": users.RoleUser}); err != nil {
		t.Fatal(err)
	}
	names := metadataNames(adapter.ConversationToolMetadata(context.Background(), task))
	if names["read_file"] || names[computerAccessToolName] || !names["calculator"] {
		t.Fatal("actor demotion did not narrow metadata", names)
	}
	profile, err := s.agentManager.SaveProfile(scope.Workspace, scope.Actor, "", 0, agents.ProfileSettings{Name: "No external tools", Model: "model", MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	current, _ := s.agentManager.Conversation(scope, receipt.Conversation.ID)
	preferences := current.Preferences
	preferences.ProfileID, preferences.ProfileRevision = profile.ID, profile.Revision
	_, err = s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "narrow-profile", TargetID: current.ID, ExpectedRevision: current.Revision, Kind: "configure_conversation", Payload: agents.ConversationCommandPayload{Preferences: &preferences}})
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.ConversationToolMetadata(context.Background(), task); len(got) != 0 {
		t.Fatal("new narrower profile ignored", got)
	}
	if err := s.userStore.DeleteUser(user.ID); err != nil {
		t.Fatal(err)
	}
	if len(adapter.ConversationToolMetadata(context.Background(), task)) != 0 {
		t.Fatal("deleted actor retained metadata")
	}
}

func TestConversationMetadataThroughActualRunner(t *testing.T) {
	s, scope, conversation := contextInboxFixture(t)
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	calls := 0
	s.agentRunner = agents.NewRunner(s.agentManager, adapter, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		found := false
		for _, tool := range tools {
			found = found || tool.Function.Name == "task_find_tools"
			if tool.Function.Name == "calculator" || tool.Function.Name == computerAccessToolName {
				t.Error("discovery exposed executable schema")
			}
		}
		if !found {
			t.Error("foreground discovery absent")
		}
		if calls == 1 {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "discovery", Type: "function", Function: api.FunctionCall{Name: "task_find_tools", Arguments: `{"query":"calculator"}`}}}}}}}, nil
		}
		var result struct {
			Tools               []capabilities.Descriptor `json:"tools"`
			MetadataOnly        bool                      `json:"metadata_only"`
			ExecutionAuthorized bool                      `json:"execution_authorized"`
		}
		if json.Unmarshal([]byte(messages[len(messages)-1].StringContent()), &result) != nil || len(result.Tools) != 1 || result.Tools[0].Name != "calculator" || !result.MetadataOnly || result.ExecutionAuthorized {
			t.Error("wrong metadata result", messages[len(messages)-1].StringContent())
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Calculator metadata is available; no calculation was performed."}}}}, nil
	})
	task := preparedMetadataTurn(t, s, scope, conversation)
	completed, err := s.agentRunner.Continue(context.Background(), task.ID, scope.Actor, "start", "", false)
	if err != nil || completed.Status != agents.TaskCompleted || calls != 2 || completed.PendingInput != nil || completed.Config.ComputerSession != "" || completed.WorkID != "" || len(completed.Checkpoint.DiscoveredTools) != 0 {
		t.Fatal("metadata performed work", completed, calls, err)
	}
}
