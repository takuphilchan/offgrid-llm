package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func (s *Server) effectiveConversationPreferences(r *http.Request, c *agents.Conversation, requestModel string) (*agents.EffectiveConversationPreferences, error) {
	scope := s.controlScope(r)
	project, err := s.agentManager.Project(scope, c.ProjectID)
	if err != nil {
		return nil, err
	}
	var profile *agents.AgentProfile
	if c.Preferences.ProfileID != "" {
		profile, err = s.agentManager.GetProfile(scope.Workspace, scope.Actor, c.Preferences.ProfileID, c.Preferences.ProfileRevision)
		if err != nil {
			return nil, err
		}
	}
	defaults := agents.ConversationPreferences{}
	if s.config != nil {
		defaults.Model = s.config.DefaultModel
	}
	user, err := s.agentManager.UserPreferences(scope)
	if err != nil {
		return nil, err
	}
	e := agents.ResolveConversationPreferences(requestModel, c.Preferences, profile, project.Preferences, agents.ConversationPreferences{Model: user.Preferences.Model, Delegation: user.Preferences.Delegation}, defaults)
	e.ConfigurationKnown = c.ConfigurationKnown
	e.UserPreferencesRevision = user.Revision
	e.ProjectID, e.ProjectRevision = project.ID, project.Revision
	if _, err = s.conversationFolderResource(r, c, &e); err != nil {
		return nil, err
	}
	resources, err := s.conversationResources(r, project, &e, profile)
	if err != nil {
		return nil, err
	}
	e.Resources = resources.resolutions
	return &e, nil
}

func (s *Server) resolveConversationRun(w http.ResponseWriter, r *http.Request, c *agents.Conversation, command agents.ConversationCommand) (agents.AgentConfig, string, bool) {
	return s.resolveConversationRunBeforeInput(w, r, c, command, "")
}

func (s *Server) resolveConversationRunBeforeInput(w http.ResponseWriter, r *http.Request, c *agents.Conversation, command agents.ConversationCommand, beforeInput string) (agents.AgentConfig, string, bool) {
	e, err := s.effectiveConversationPreferences(r, c, command.Payload.Input.Model)
	if err != nil {
		writeControlError(w, err)
		return agents.AgentConfig{}, "", false
	}
	if e.Model == "" {
		writeJobError(w, 422, "model_unavailable", "Choose an installed model for this conversation.", false)
		return agents.AgentConfig{}, "", false
	}
	req := jobSubmitRequest{ConversationModelOverride: true, Prompt: command.Payload.Input.Content, Model: e.Model, ProfileID: e.ProfileID, ProfileRevision: e.ProfileRevision}
	if e.ProfileID != "" {
		profile, loadErr := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), e.ProfileID, e.ProfileRevision)
		if loadErr != nil {
			writeControlError(w, loadErr)
			return agents.AgentConfig{}, "", false
		}
		req.Restrictions = inheritConversationRestrictions(e.Restrictions, nil, profile)
	}
	config, ok := s.resolveJobConfig(w, r, &req)
	if !ok {
		return config, "", false
	}
	config.DisableDelegation = !e.Delegation
	// The conversation preview owns admission of the durable clarification
	// protocol. Pin it for new generic-tool runs; native adapters still require
	// their own qualification and existing runs retain their recorded version.
	if s.config != nil && s.config.EnableConversationPreview && !config.TextOnlyInference && config.ToolAdapter == nil {
		config.WorkerQuestionsVersion = 1
	}
	config.ProjectID, config.ProjectRevision = e.ProjectID, e.ProjectRevision
	config.ConversationContextRevision = c.ContextRevision
	config.EffectivePreferences = e
	if e.ProfileID == "" && !config.TextOnlyInference {
		config.ScopedCapabilities = true
		config.AllowedCapabilities = map[string]capabilities.Descriptor{}
		for name, descriptor := range s.profileToolCatalog(r) {
			if e.Restrictions.Tools != nil {
				ref := agents.ProfileToolReference{Provider: descriptor.Source, Tool: name, SchemaRevision: descriptor.Revision, Location: descriptor.Location}
				if !profileContainsTool(*e.Restrictions.Tools, ref) {
					continue
				}
			}
			config.AllowedCapabilities[name] = descriptor
		}
		if e.Restrictions.Computer != nil {
			config.RestrictComputer = !*e.Restrictions.Computer
		}
		config.ExecutionPolicy = e.Restrictions.Execution
	}
	if e.ProfileID == "" && e.Restrictions.Knowledge != nil {
		config.KnowledgeSelection = e.Restrictions.Knowledge
		if !s.attachJobKnowledge(w, r, &req, &config) {
			return config, "", false
		}
	}
	config.ConversationConfiguration = &agents.ConversationConfiguration{Version: 1, Profile: config.Profile, Model: req.Model, Restrictions: e.Restrictions}
	if err = s.attachConversationFolder(r, c, e, &config); err != nil {
		writeControlError(w, err)
		return config, "", false
	}
	if !s.pinConversationResources(w, r, e, &config, &req) {
		return config, "", false
	}
	if err = s.assembleCanonicalHistory(r, c, beforeInput, &config); err != nil {
		writeControlError(w, err)
		return config, "", false
	}
	return config, req.Model, true
}
