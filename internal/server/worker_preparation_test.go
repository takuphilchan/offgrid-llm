package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalWorkerPreparationStorageBindingAndStop(t *testing.T) {
	s := profileFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "input.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	binding := execution.Binding{Actor: scope.Actor, Workspace: scope.Workspace, Host: "host", Session: "session", Grant: "grant", Revision: 1, Job: "supervisor"}
	config := execution.Context{Version: 1, Binding: binding, Backend: "local", Root: folder, Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}, WorkerCopies: execution.DefaultWorkingCopyPolicy()}
	session, err := execution.OpenSession(config)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := execution.OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := execution.OpenWorkingCopies(filepath.Join(t.TempDir(), "copies"), config.WorkerCopies)
	if err != nil {
		t.Fatal(err)
	}
	worker := &execution.Worker{Session: session, Journal: journal, Processes: execution.NewProcessManager(session), WorkingCopies: store, Token: strings.Repeat("a", 64)}
	endpoint := execution.NewLocalEndpoint(worker)
	t.Cleanup(func() { endpoint.Close(); store.Close(); journal.Close(); session.Close() })
	s.executionHosts = execution.NewHosts()
	if err = s.executionHosts.RegisterApproved(ctx, config, endpoint); err != nil {
		t.Fatal(err)
	}
	ref := execution.Reference{Host: binding.Host, Session: binding.Session, Grant: binding.Grant, Revision: 1}
	resource, err := s.executionHosts.Resolve(ctx, scope.Actor, scope.Workspace, ref)
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	s.agentRunner = agents.NewRunner(s.agentManager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		call := api.ToolCall{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"review","goal":"Review inputs","depends_on":[],"tools":[]}]}`}}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}}}}, nil
	})
	defer s.agentRunner.Shutdown(context.Background())
	conversation, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ScopedCapabilities = true, true
	cfg.ContextWindow = 32768
	cfg.Execution, cfg.ExecutionWorkspace = &ref, &resource
	accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: conversation.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Review this"}}}, "model", cfg)
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.agentRunner.Continue(ctx, accepted.Work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatal(root, err)
	}
	binding.Job = root.ID
	req := execution.WorkingCopyRequest{RequestID: "copy", Parent: binding, WorkerRunID: root.ChildHistory[0].ID, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch, Permissions: config.Permissions, MaxBytes: 1024}
	status, err := tools.BeginExecutionWorker(ctx, root, req)
	if err != nil {
		t.Fatal(err)
	}
	for status.State == "accepted" {
		status, err = tools.InspectExecutionWorker(ctx, root, req)
		if err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
		time.Sleep(time.Millisecond)
	}
	if status.State != "ready" {
		t.Fatal(status)
	}
	saved, _ := s.agentManager.GetTask(root.ID)
	if saved.Checkpoint.StorageBudget == nil || saved.Checkpoint.StorageBudget.ChargedBytes != 1024 {
		t.Fatal("missing durable root reservation")
	}
	child, _ := s.agentManager.GetTask(req.WorkerRunID)
	if child.Config.Execution != nil {
		t.Fatal("observation admitted child")
	}
	childResource, err := tools.AdmitExecutionWorker(ctx, root, req)
	if err != nil {
		t.Fatal(err)
	}
	child, _ = s.agentManager.GetTask(req.WorkerRunID)
	if child.Config.Execution == nil || *child.Config.Execution != childResource.Reference || child.Checkpoint.WorkerEnvironment == nil || !agents.HasOwnedExecutionProcesses(child) {
		t.Fatal("canonical child missing independent binding")
	}
	if _, err = tools.AdmitExecutionWorker(ctx, root, req); err != nil {
		t.Fatal("exact admission retry", err)
	}
	stopped, err := s.agentRunner.Stop(child.ID, scope.Actor, "cancel", "")
	if err != nil || stopped.Status != agents.TaskCancelled {
		t.Fatal(stopped, err)
	}
	if _, err = tools.AdmitExecutionWorker(ctx, root, req); err == nil {
		t.Fatal("stopped canonical child readmitted")
	}
	if _, err = s.executionHosts.Call(ctx, scope.Actor, scope.Workspace, child.ID, "late", childResource.Reference, "workspace_info", []byte(`{}`)); err == nil {
		t.Fatal("stopped child dispatched")
	}
	if _, err = s.executionHosts.Resolve(ctx, scope.Actor, scope.Workspace, ref); err != nil {
		t.Fatal("child Stop revoked parent", err)
	}
	stopped, err = s.agentRunner.Stop(root.ID, scope.Actor, "cancel", "")
	if err != nil || stopped.Status != agents.TaskCancelled {
		t.Fatal(stopped, err)
	}
	if stopped.Checkpoint.StorageBudget.ChargedBytes != 1024 {
		t.Fatal("Stop refunded retained files")
	}
}
