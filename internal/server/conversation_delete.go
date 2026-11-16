package server

import (
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"net/http"
	"strconv"
)

// Explicit canonical identity/revision opt-in preserves the legacy write fence.
func (s *Server) deleteCanonicalConversation(w http.ResponseWriter, r *http.Request, name string, manifest *agents.ConversationImportManifest) {
	id := r.URL.Query().Get("canonical_id")
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	choice := r.URL.Query().Get("work")
	if id != manifest.ConversationID {
		writeControlError(w, agents.ErrControlNotFound)
		return
	}
	if err != nil || revision < 1 || (choice != "" && choice != "retain" && choice != "stop") {
		writeControlError(w, agents.ErrControlCommand)
		return
	}
	unlock, available := s.sessionHandlers.tryLockSession(name)
	if !available {
		writeSessionError(w, sessions.ErrTurnConflict)
		return
	}
	defer unlock()
	scope := s.controlScope(r)
	deleted, err := s.agentManager.ConversationDeleted(scope, id)
	if err != nil {
		writeControlError(w, err)
		return
	}
	if !deleted {
		current, err := s.agentManager.Conversation(scope, id)
		if err != nil {
			writeControlError(w, err)
			return
		}
		if current.Revision != revision {
			writeControlError(w, agents.ErrRunConflict)
			return
		}
		runs, err := s.agentManager.ConversationRuns(scope, id)
		if err != nil {
			writeControlError(w, err)
			return
		}
		runs = append(runs, manifest.LegacyRunIDs...)
		for _, run := range runs {
			task, found := s.agentManager.GetTask(run)
			if !found {
				continue
			}
			if task.Actor != scope.Actor {
				writeControlError(w, agents.ErrControlNotFound)
				return
			}
			active := task.Status != agents.TaskCompleted && task.Status != agents.TaskFailed && task.Status != agents.TaskCancelled
			if active && choice == "" {
				writeJobError(w, 409, "conversation_has_work", "Choose whether to stop active work or retain it in Activity before deleting.", false)
				return
			}
			if active && choice == "stop" {
				if s.agentRunner == nil {
					writeJobReadError(w, agents.ErrRunStorage)
					return
				}
				task, err = s.agentRunner.Stop(task.ID, scope.Actor, "cancel", "")
				if err != nil {
					writeJobCommandError(w, err)
					return
				}
				if s.browserHub != nil {
					s.browserHub.StopTask(scope.Actor, task.ID)
				}
			}
			if choice == "stop" && (task.Status == agents.TaskUncertain || (s.agentRunner != nil && s.agentRunner.WorkerActive(task.ID))) {
				writeJobError(w, 409, "task_stop_pending", "Work has not safely settled. Wait or retain it in Activity before deleting.", true)
				return
			}
		}
	}
	err = s.sessionHandlers.scoped(r).DeleteCanonical(name, func() error {
		return s.agentManager.DeleteImportedConversation(scope, name, id, revision, choice == "retain")
	})
	switch {
	case errors.Is(err, agents.ErrConversationHasWork):
		writeJobError(w, 409, "conversation_has_work", "Choose whether to stop active work or retain it in Activity before deleting.", false)
	case errors.Is(err, agents.ErrConversationInputPending):
		writeJobError(w, 409, "conversation_input_pending", "Accepted input is still pending. Open the conversation to stop or finish it, then retry deletion.", true)
	case err != nil:
		writeControlError(w, err)
	default:
		writeJSON(w, 200, map[string]any{"success": true, "work_retained": true})
	}
}
