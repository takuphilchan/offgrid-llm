package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationDeliveryDefersUnavailableModelAndSerializesTurns(t *testing.T) {
	s := profileFixture(t)
	directory := t.TempDir()
	s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, directory)
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "new", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Conversation
	s.config.DefaultModel = ""
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Answer"}}}}, nil
	})
	first := acceptContextInput(t, s, scope, c.ID, "first", "First question")
	second := acceptContextInput(t, s, scope, c.ID, "second", "Second question")
	controller := &conversationDeliveryController{server: s}
	now := time.Now()
	if err := controller.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	deferred, err := s.agentManager.ConversationInput(scope, c.ID, first.ID)
	if err != nil || deferred.DeliveryIssue == nil || deferred.DeliveryIssue.Code != "model_unavailable" || deferred.State != "accepted" || deferred.RunID != "" || calls.Load() != 0 {
		t.Fatal(deferred, err)
	}
	s.config.DefaultModel = "model"
	if err = controller.coordinate(context.Background(), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := s.agentManager.ConversationInput(scope, c.ID, first.ID)
	if unchanged.RunID != "" || unchanged.DeliveryIssue.RetryAt != deferred.DeliveryIssue.RetryAt {
		t.Fatal("retry backoff ignored", unchanged)
	}
	if err = controller.coordinate(context.Background(), now.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	delivered, _ := s.agentManager.ConversationInput(scope, c.ID, first.ID)
	if delivered.RunID == "" || delivered.DeliveryIssue != nil || calls.Load() != 0 {
		t.Fatal("preparation did not clear limitation or dispatched inference", delivered)
	}
	if err = controller.coordinate(context.Background(), now.Add(32*time.Second)); err != nil {
		t.Fatal(err)
	}
	queued, _ := s.agentManager.ConversationInput(scope, c.ID, second.ID)
	if queued.RunID != "" {
		t.Fatal("overlapping conversation turns", queued)
	}
	if _, err = s.agentRunner.Continue(context.Background(), delivered.RunID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	if err = controller.coordinate(context.Background(), now.Add(33*time.Second)); err != nil {
		t.Fatal(err)
	}
	queued, _ = s.agentManager.ConversationInput(scope, c.ID, second.ID)
	if queued.RunID == "" || queued.RunID == delivered.RunID || calls.Load() != 1 {
		t.Fatal(queued, calls.Load())
	}
	// Restart interrupts the pinned run. Scanning accepted inputs cannot revive
	// that run or create a replacement, even after its original retry time.
	s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, directory)
	if err = s.agentManager.StorageError(); err != nil {
		t.Fatal(err)
	}
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, nil)
	controller = &conversationDeliveryController{server: s}
	if err = controller.coordinate(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	recovered, _ := s.agentManager.ConversationInput(scope, c.ID, second.ID)
	if recovered.RunID != queued.RunID || recovered.RunStatus != agents.TaskInterrupted {
		t.Fatal("restart redispatched input", recovered)
	}
}

func TestConversationDeliveryUsesRunnerLifetimeWithoutClient(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.DefaultModel = "model"
	var calls atomic.Int32
	s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Service-owned reply"}}}}, nil
	})
	controller := &conversationDeliveryController{server: s}
	s.agentRunner.Coordinate = controller.coordinate
	s.agentRunner.SourceAccess = s.taskSourceAllowed
	service, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer s.agentRunner.Shutdown(context.Background())
	s.agentRunner.StartCoordinator(service)
	input := acceptContextInput(t, s, scope, c.ID, "input", "Hello")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current, err := s.agentManager.ConversationInput(scope, c.ID, input.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.State == "responded" {
			if calls.Load() != 1 {
				t.Fatal("duplicate execution", calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("service-owned input did not settle")
}
