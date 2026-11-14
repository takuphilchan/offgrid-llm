package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputPublicServiceJourneyWithoutProfile(t *testing.T) {
	for _, ongoing := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary-answer", true: "ongoing-work"}[ongoing], func(t *testing.T) {
			s, scope, c := contextInboxFixture(t)
			s.config.DefaultModel, s.config.EnableConversationPreview = "model", true
			var calls atomic.Int32
			s.agentRunner = agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				calls.Add(1)
				if task.Config.Profile != nil {
					t.Error("default input manufactured a profile")
				}
				if ongoing {
					return foregroundWorkModel(ctx, task, messages, tools)
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Ordinary answer"}}}}, nil
			})
			controller := &conversationDeliveryController{server: s}
			s.agentRunner.Coordinate, s.agentRunner.SourceAccess = controller.coordinate, s.taskSourceAllowed
			service, cancel := context.WithCancel(context.Background())
			defer cancel()
			defer s.agentRunner.Shutdown(context.Background())
			s.agentRunner.StartCoordinator(service)
			server := httptest.NewServer(http.HandlerFunc(s.handleConversations))
			defer server.Close()
			client, err := serviceclient.New(server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := client.SubmitConversationInput(context.Background(), api.ConversationInputCommand{RequestID: "public-input", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Compare these approaches"}})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(6 * time.Second)
			for time.Now().Before(deadline) {
				entry, err := client.ConversationInput(context.Background(), c.ID, receipt.Input.ID)
				if err != nil {
					t.Fatal(err)
				}
				if entry.AppliedAt != nil {
					if !ongoing {
						if entry.Incorporation != "answered" || entry.CreatedWorkID != "" || calls.Load() != 1 {
							t.Fatal("simple answer activated work or a classifier", entry, calls.Load())
						}
						return
					}
					work, err := s.agentManager.WorkItem(scope, entry.CreatedWorkID)
					if err != nil {
						t.Fatal(err)
					}
					if work.Status == agents.TaskCompleted {
						if calls.Load() != 3 {
							t.Fatal("duplicate work or classifier call", calls.Load())
						}
						return
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("public input did not reach its service-owned outcome")
		})
	}
}
