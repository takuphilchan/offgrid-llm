package server

import (
	"encoding/json"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func (s *Server) bindCanonicalConversationGuard() {
	if s.sessionHandlers == nil || s.agentManager == nil {
		return
	}
	s.sessionHandlers.manager.SetCanonicalGuard(func(session *sessions.Session) error {
		actor := session.OwnerID
		if !s.sessionHandlers.requireAuth || actor == "" {
			actor = "local-admin"
		}
		manifest, err := s.agentManager.ImportedConversation(agents.ControlScope{Workspace: s.workspaceID, Actor: actor}, session.Name)
		if err != nil {
			return err
		}
		if manifest != nil {
			deleted, err := s.agentManager.ConversationDeleted(agents.ControlScope{Workspace: s.workspaceID, Actor: actor}, manifest.ConversationID)
			if err != nil {
				return err
			}
			if deleted {
				return sessions.ErrSessionNotFound
			}
			return sessions.ErrCanonicalConversation
		}
		return nil
	})
}

// migrateConversation is called only by an explicit mutation, never navigation
// or a GET. The old session lock and storage mutation lock cover the ownership
// switch. No source file, original attachment, or historical job is removed.
func (s *Server) migrateConversation(r *http.Request, name string) (*agents.ConversationImportManifest, error) {
	return s.migrateConversationWith(r, name, func(input agents.ConversationImport) (*agents.ConversationImportManifest, error) {
		return s.agentManager.ImportConversation(s.controlScope(r), input)
	})
}

func (s *Server) migrateConversationWith(r *http.Request, name string, commit func(agents.ConversationImport) (*agents.ConversationImportManifest, error)) (*agents.ConversationImportManifest, error) {
	h := s.sessionHandlers
	if h == nil || s.agentManager == nil {
		return nil, agents.ErrRunStorage
	}
	unlock, err := h.lockSessionContext(r.Context(), name)
	if err != nil {
		return nil, err
	}
	defer unlock()
	var manifest *agents.ConversationImportManifest
	err = h.scoped(r).MigrateSnapshot(name, func(session *sessions.Session, source []byte) error {
		actor := s.agentActor(r)
		if h.requireAuth && session.OwnerID != actor {
			return sessions.ErrSessionNotFound
		}
		input := agents.ConversationImport{LegacyName: name, Source: source, CreatedAt: session.CreatedAt, Preferences: agents.ConversationPreferences{Model: session.ModelID}}
		for _, message := range session.Messages {
			data, e := json.Marshal(message)
			if e != nil {
				return e
			}
			input.Messages = append(input.Messages, data)
		}
		for _, link := range session.Jobs {
			task, found := s.agentManager.GetTask(link.JobID)
			if !found || task.DeletedAt != nil {
				continue
			}
			if task.Actor != actor || task.Origin == nil || task.Origin.Conversation != name || task.Origin.Workspace != s.workspaceID {
				return agents.ErrControlNotFound
			}
			if task.Status != agents.TaskCompleted && task.Status != agents.TaskFailed && task.Status != agents.TaskCancelled {
				return agents.ErrRunConflict
			}
			input.LegacyRunIDs = append(input.LegacyRunIDs, task.ID)
			if config := task.Config.ConversationConfiguration; config != nil && session.Turn != nil && session.Turn.JobID == task.ID {
				if input.Preferences.Model == "" {
					input.Preferences.Model = config.Model
				}
				input.Preferences.Restrictions = config.Restrictions
				if config.Profile != nil {
					input.Preferences.ProfileID = config.Profile.ID
					input.Preferences.ProfileRevision = config.Profile.Revision
				}
				input.ConfigurationKnown = true
			}
		}
		var e error
		manifest, e = commit(input)
		return e
	})
	return manifest, err
}

func (s *Server) importConversationCommand(r *http.Request, command agents.ConversationCommand) (*agents.ConversationCommandReceipt, error) {
	var receipt *agents.ConversationCommandReceipt
	_, err := s.migrateConversationWith(r, command.Payload.LegacyName, func(input agents.ConversationImport) (*agents.ConversationImportManifest, error) {
		var err error
		receipt, err = s.agentManager.ImportConversationCommand(s.controlScope(r), command, input)
		if err != nil {
			return nil, err
		}
		return receipt.Import, nil
	})
	return receipt, err
}
