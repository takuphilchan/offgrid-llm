package server

import (
	"context"
	"log"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// AgentRuntimeHooks exposes only the transport-dependent callbacks needed to
// compose a runner in app. The transport does not acquire or own the runner.
type AgentRuntimeHooks struct {
	Tools         agents.RunTools
	Call          agents.RunCaller
	Stream        agents.RunStreamCaller
	ContextWindow func(*agents.Task) int
	CountContext  func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (int, error)
	Observe       func(*agents.Task)
}

func (s *Server) AgentHooks() AgentRuntimeHooks {
	return AgentRuntimeHooks{
		Tools:         &browserRunTools{RunTools: s.toolRegistry, server: s},
		Call:          s.callAgentModel,
		Stream:        s.streamAgentModel,
		ContextWindow: s.taskContextWindow,
		CountContext:  s.countAgentContext,
		Observe: func(task *agents.Task) {
			s.projectConversationTask(task)
			if task.Status == agents.TaskCompleted || task.Status == agents.TaskFailed || task.Status == agents.TaskCancelled {
				s.browserHub.StopTask(task.Actor, task.ID)
			}
			if task.Status == agents.TaskInput && task.PendingInput != nil {
				s.browserHub.StopSession(task.Actor, task.PendingInput.PreviousSession)
			}
			s.publishRunEvent(context.Background(), task.ID, runs.RunStateChanged, map[string]any{"status": task.Status, "prompt": task.Prompt, "model": task.Model, "actor": task.Actor, "pending_approval": task.PendingApproval})
			if task.Status == agents.TaskCompleted {
				if _, err := s.persistRunOutput(context.Background(), task.ID, task.Result); err != nil {
					log.Printf("Agent output artifact unavailable (result retained in task): %v", err)
				}
			}
		},
	}
}

// BindApplicationRuntime restores the existing admission, download and
// session projections after app has constructed their concrete owners.
func (s *Server) BindApplicationRuntime(runner *agents.Runner) error {
	if runner == nil {
		return newServiceError(http.StatusInternalServerError, "agent runner is required", nil)
	}
	if err := s.providerScheduler.ConfigureLegacyRetirement(s.retireLegacyForNative); err != nil {
		return err
	}
	s.agentRunner = runner
	s.sessionHandlers.requireAuth = s.config.RequireAuth
	s.sessionHandlers.agentServer = s
	if err := s.loadDownloads(); err != nil {
		return err
	}
	s.sessionHandlers.streamer = s.streamSessionChat
	s.sessionHandlers.runtimeContext = s.runtimeCtx
	s.browserHub.RecordReply = s.recordComputerReply
	if err := s.recoverConversationAdmissions(); err != nil {
		return err
	}
	s.sessionHandlers.SetCompleter(func(ctx context.Context, modelID string, messages []api.ChatMessage, useKnowledgeBase bool) (string, error) {
		response, err := s.completeChat(ctx, &api.ChatCompletionRequest{Model: modelID, Messages: messages, UseKnowledgeBase: &useKnowledgeBase})
		if err != nil {
			return "", err
		}
		if len(response.Choices) == 0 {
			return "", newServiceError(http.StatusBadGateway, "model returned no response", nil)
		}
		answer := response.Choices[0].Message.StringContent()
		if answer == "" {
			return "", newServiceError(http.StatusBadGateway, "model returned an empty response", nil)
		}
		return answer, nil
	})
	return nil
}
