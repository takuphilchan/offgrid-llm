package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func (s *Server) validateConversationSelection(w http.ResponseWriter, r *http.Request, c agents.ConversationCommand) bool {
	if c.Kind != "link_work" && c.Kind != "focus_work" || c.Payload.WorkID == "" {
		return true
	}
	work, err := s.agentManager.WorkItem(s.controlScope(r), c.Payload.WorkID)
	if err != nil {
		writeControlError(w, err)
		return false
	}
	task, found := s.agentManager.GetTask(work.RunID)
	if !found || task.Actor != s.agentActor(r) || task.DeletedAt != nil {
		writeControlError(w, agents.ErrControlNotFound)
		return false
	}
	return !s.rejectUnavailableTask(w, r, task)
}

func (s *Server) conversationContainsTask(actor, conversation string, task *agents.Task) bool {
	if task.Actor != actor || task.Origin == nil || task.Origin.Workspace != s.workspaceID {
		return false
	}
	if task.Origin.Conversation == conversation {
		return true
	}
	if task.WorkID == "" || task.ParentID != "" {
		return false
	}
	linked, err := s.agentManager.ConversationContainsRun(agents.ControlScope{Workspace: s.workspaceID, Actor: actor}, conversation, task.WorkID, task.ID)
	return err == nil && linked
}
