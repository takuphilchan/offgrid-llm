package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// This is an observation projection. Its question identity, not the read, is
// bound to the later input command. Stale replies cannot answer a newer question.
func (s *Server) pendingConversationQuestion(r *http.Request, conversation *agents.Conversation) *api.ConversationQuestion {
	if conversation.FocusedWorkID == "" {
		return nil
	}
	scope := s.controlScope(r)
	work, err := s.agentManager.WorkItem(scope, conversation.FocusedWorkID)
	if err != nil {
		return nil
	}
	linked, err := s.agentManager.ConversationContainsRun(scope, conversation.ID, work.ID, work.RunID)
	if err != nil || !linked {
		return nil
	}
	root, ok := s.agentManager.GetTask(work.RunID)
	if !ok || root.Actor != scope.Actor || root.DeletedAt != nil || root.Status != agents.TaskInput || root.PendingInput == nil || root.PendingInput.Kind != "worker_clarification" || root.ControlAction != "" || root.ControlPending || root.ConversationHold != nil || !s.taskSourceAllowed(root) {
		return nil
	}
	return &api.ConversationQuestion{ID: root.PendingInput.ID, WorkID: work.ID, RunID: root.ID, Content: root.PendingInput.Target}
}

func (s *Server) conversationQuestionSourceAllowed(scope agents.ControlScope, question *api.ConversationQuestion) bool {
	if question == nil {
		return false
	}
	root, ok := s.agentManager.GetTask(question.RunID)
	return ok && root.Actor == scope.Actor && root.DeletedAt == nil && root.WorkID == question.WorkID && root.Origin != nil && root.Origin.Workspace == scope.Workspace && s.taskSourceAllowed(root)
}
