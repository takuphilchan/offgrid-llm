package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Prepare metadata outside runner/store locks. The runner rechecks the saved
// selection version and commits this binding with the accepted access input.
func (s *Server) prepareExecutionProject(w http.ResponseWriter, r *http.Request, task *agents.Task, resource execution.Resource) (*agents.ExecutionProjectContext, bool) {
	if !resource.ProjectContext || task.WorkID == "" || task.Origin == nil || task.ConversationTurn != nil {
		return nil, true
	}
	if task.Actor != s.agentActor(r) || task.Origin.Workspace != s.workspaceID {
		writeControlError(w, agents.ErrControlNotFound)
		return nil, false
	}
	// Conversation preferences can change while the run waits. Discovery must
	// satisfy both today's restrictions and the authority ceiling pinned to it.
	read, _ := execution.Descriptor("workspace_read")
	if task.Config.ExecutionPolicy != nil && !task.Config.ExecutionPolicy.Read ||
		task.Config.Profile != nil && task.Config.Profile.Settings.Execution != nil && !task.Config.Profile.Settings.Execution.Read ||
		task.Config.ConversationConfiguration != nil && !restrictionAllowsDescriptor(task.Config.ConversationConfiguration.Restrictions, read) {
		return nil, true
	}
	c, err := s.agentManager.Conversation(s.controlScope(r), task.Origin.Conversation)
	if err != nil {
		writeControlError(w, err)
		return nil, false
	}
	if !agents.ExecutionProjectSelectionCurrent(task, c) {
		return nil, true
	}
	prefs, err := s.effectiveConversationPreferences(r, c, "")
	if err != nil {
		writeControlError(w, err)
		return nil, false
	}
	allowed, err := s.projectFolderAllowed(r, prefs)
	if err != nil {
		writeControlError(w, err)
		return nil, false
	}
	if !allowed {
		return nil, true
	}
	observed, err := s.executionHosts.ProjectContext(r.Context(), task.Actor, s.workspaceID, resource.Reference)
	if err != nil {
		writeJobError(w, 409, "project_folder_unavailable", "The approved folder's project context could not be read. Reconnect its host and check .offgrid/project.toml; work is still waiting for access.", false)
		return nil, false
	}
	if observed.Present {
		if _, err = agents.ParseProjectFile(observed.Configuration); err != nil {
			writeJobError(w, 422, "project_configuration_invalid", "The approved folder's .offgrid/project.toml is invalid. Correct its version 1 preferences/resource references; no access input or project binding was committed.", false)
			return nil, false
		}
	}
	return &agents.ExecutionProjectContext{Snapshot: observed}, true
}
