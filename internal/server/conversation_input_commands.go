package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleSubmitConversationInput(w http.ResponseWriter, r *http.Request, conversation string) {
	var command api.ConversationInputCommand
	if decodeControlEnvelope(w, r, &command) != nil || command.TargetID != conversation {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	scope := s.controlScope(r)
	previous, err := s.agentManager.ConversationInputCommandReceipt(scope, command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if previous != nil {
		writeJSON(w, 200, s.publicInputReceipt(r, previous))
		return
	}
	current, err := s.agentManager.Conversation(scope, conversation)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if current.Revision != command.ExpectedRevision {
		writeControlError(w, agents.ErrRunConflict)
		return
	}
	if command.Payload.Intent != "answer" && (s.config == nil || !s.config.EnableConversationPreview) {
		writeJobError(w, 503, "conversation_admission_disabled", "New conversation input is disabled. Saved input and work controls remain available.", false)
		return
	}
	if s.agentRunner == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	focus := current.FocusedWorkID
	if command.Payload.FocusedWorkID != nil {
		focus = *command.Payload.FocusedWorkID
	}
	if command.Payload.Intent == "new_work" {
		focus = ""
	}
	if focus != "" {
		work, err := s.agentManager.WorkItem(scope, focus)
		if err != nil {
			writeControlError(w, err)
			return
		}
		linked, err := s.agentManager.ConversationContainsRun(scope, conversation, focus, work.RunID)
		if err != nil || !linked {
			writeControlError(w, agents.ErrControlNotFound)
			return
		}
		task, found := s.agentManager.GetTask(work.RunID)
		if !found || task.Actor != scope.Actor || task.DeletedAt != nil {
			writeControlError(w, agents.ErrControlNotFound)
			return
		}
		if !s.taskSourceAllowed(task) && (command.Payload.Intent == "answer" || command.Payload.Intent == "correction") {
			writeJobError(w, 403, "conversation_context_unavailable", "The focused work uses sources that are no longer accessible. Select authorized work or send an unbound message.", false)
			return
		}
	}
	receipt, err := s.agentRunner.AcceptConversationInput(scope, command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v2/conversations/"+conversation+"/inputs/"+receipt.Input.ID)
	writeJSON(w, http.StatusAccepted, s.publicInputReceipt(r, receipt))
}

func (s *Server) handleConversationInputControl(w http.ResponseWriter, r *http.Request, conversation, input string) {
	var command api.ConversationInputControlCommand
	if decodeControlEnvelope(w, r, &command) != nil || command.TargetID != input {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	previous, err := s.agentManager.ConversationInputControlReceipt(s.controlScope(r), conversation, command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if previous != nil {
		writeJSON(w, 200, s.publicInputReceipt(r, previous))
		return
	}
	if s.agentRunner == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	receipt, err := s.agentRunner.ControlConversationInput(s.controlScope(r), conversation, command)
	if err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, s.publicInputReceipt(r, receipt))
}
