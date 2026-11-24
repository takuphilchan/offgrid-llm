package server

import (
	"context"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Preparation reuses the same configuration/authority resolver as explicit
// work commands. It does not copy the foreground or focused work's grants.
func (s *Server) prepareConversationInputWork(ctx context.Context, scope agents.ControlScope, conversation, inputID string) (*api.ConversationInboxEntry, error) {
	if s.agentManager == nil || s.agentRunner == nil {
		return nil, agents.ErrRunStorage
	}
	ctx, err := s.conversationActorContext(ctx, scope)
	if err != nil {
		return nil, err
	}
	input, err := s.agentManager.ConversationInput(scope, conversation, inputID)
	if err != nil {
		return nil, err
	}
	if input.Decision == nil || input.Decision.Kind != "new_work" {
		return nil, agents.ErrControlCommand
	}
	if input.AppliedAt != nil {
		return s.agentRunner.AdmitConversationInputWork(scope, conversation, inputID, 0, "", agents.AgentConfig{})
	}
	foreground, exists := s.agentManager.GetTask(input.RunID)
	if !exists || !s.taskSourceAllowed(foreground) {
		return nil, agents.ErrSourceAccess
	}
	if s.registry == nil {
		return nil, newCodedServiceError(503, "model_catalog_unavailable", "The work decision is saved; the installed model catalog is unavailable.", nil)
	}
	current, err := s.agentManager.Conversation(scope, conversation)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v2/conversations/"+conversation, nil)
	if err != nil {
		return nil, err
	}
	command := agents.ConversationCommand{Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: input.Input.Content, Model: input.Input.Model}}}
	response := &preparationResponse{header: http.Header{}}
	config, model, ok := s.resolveConversationRunBeforeInput(response, request, current, command, inputID, input.Input)
	if !ok {
		return nil, response.preparationError()
	}
	if err = s.attachConversationInputSources(request, current, input.Input, &config); err != nil {
		return nil, err
	}
	if err = s.appendFocusedWorkContext(scope, conversation, input, &config); err != nil {
		return nil, err
	}
	config.ConversationContext = append(config.ConversationContext, api.ChatMessage{Role: "assistant", Content: "Recorded proposed acceptance condition for the current user request (not new authority or proof of completion): " + input.Decision.ExpectedResult})
	// The expected result is model-derived from the foreground's context. Its
	// source dependencies survive even if current preferences select other data.
	config.KnowledgeDependencies = append(config.KnowledgeDependencies, foreground.Config.KnowledgeDependencies...)
	for _, source := range foreground.Config.KnowledgeSources {
		config.KnowledgeDependencies = append(config.KnowledgeDependencies, source.DocumentID)
	}
	for _, document := range foreground.Config.KnowledgeDocuments {
		config.KnowledgeDependencies = append(config.KnowledgeDependencies, document.ID)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if !s.scheduleActorPermission(scope.Actor, users.PermissionChat) || !s.taskSourceAllowed(foreground) {
		return nil, agents.ErrSourceAccess
	}
	return s.agentRunner.AdmitConversationInputWork(scope, conversation, inputID, current.Revision, model, config)
}

func (s *Server) conversationActorContext(ctx context.Context, scope agents.ControlScope) (context.Context, error) {
	if scope.Workspace != s.workspaceID {
		return nil, agents.ErrControlNotFound
	}
	if s.config != nil && s.config.RequireAuth {
		if s.userStore == nil {
			return nil, agents.ErrControlNotFound
		}
		resolved, ok := s.userStore.ContextForActor(ctx, scope.Actor, users.PermissionChat)
		if !ok {
			return nil, newCodedServiceError(403, "actor_permission_revoked", "Conversation work requires current chat access.", nil)
		}
		return resolved, nil
	}
	if scope.Actor != "local-admin" {
		return nil, agents.ErrControlNotFound
	}
	return ctx, nil
}
