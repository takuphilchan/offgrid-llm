package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func (s *Server) applyProjectFolder(w http.ResponseWriter, r *http.Request, command agents.ConversationCommand) (*agents.ConversationCommandReceipt, error) {
	c, err := s.agentManager.Conversation(s.controlScope(r), command.TargetID)
	if err != nil {
		return nil, err
	}
	if c.Revision != command.ExpectedRevision {
		return nil, agents.ErrRunConflict
	}
	prefs, err := s.effectiveConversationPreferences(r, c, "")
	if err != nil {
		return nil, err
	}
	allowed, err := s.projectFolderAllowed(r, prefs)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, agents.ErrSourceAccess
	}
	folder := command.Payload.Folder
	ref := execution.Reference{Host: folder.Host, Session: folder.Session, Grant: folder.Grant, Revision: folder.Revision}
	observed, err := s.executionHosts.ProjectContext(r.Context(), s.agentActor(r), s.workspaceID, ref)
	if err != nil {
		writeJobError(w, 409, "project_folder_unavailable", "Project context could not be read from the approved folder. Check host availability, read access, and .offgrid/project.toml (a regular file of at most 64 KiB).", false)
		return nil, nil
	}
	if observed.Present {
		if _, err = agents.ParseProjectFile(observed.Configuration); err != nil {
			writeJobError(w, 422, "project_configuration_invalid", "The approved folder's .offgrid/project.toml is invalid. Use version 1 preferences and resource references; grants, credentials and executable hooks are not supported.", false)
			return nil, nil
		}
	}
	return s.agentManager.ApplyProjectFolder(s.controlScope(r), command, observed)
}

func (s *Server) projectFolderAllowed(r *http.Request, prefs *agents.EffectiveConversationPreferences) (bool, error) {
	d, _ := execution.Descriptor("workspace_read")
	if !s.profileActorCapabilityAllowed(s.agentActor(r), d) || !restrictionAllowsDescriptor(prefs.Restrictions, d) || prefs.Restrictions.Execution != nil && !prefs.Restrictions.Execution.Read || s.executionHosts == nil {
		return false, nil
	}
	if prefs.ProfileID != "" {
		profile, err := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), prefs.ProfileID, prefs.ProfileRevision)
		if err != nil {
			return false, err
		}
		if profile.Settings.Execution != nil && !profile.Settings.Execution.Read {
			return false, nil
		}
	}
	return true, nil
}
