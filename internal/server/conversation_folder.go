package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) conversationFolderResource(r *http.Request, c *agents.Conversation, e *agents.EffectiveConversationPreferences) (*execution.Resource, error) {
	ref, err := s.agentManager.ConversationFolder(s.controlScope(r), c.ID)
	if err != nil || ref == nil {
		return nil, err
	}
	e.Folder = &api.ConversationFolderStatus{Session: ref.Session, Reason: "The selected folder is unavailable. Reconnect and approve it locally to use its files; project membership does not restore access."}
	d, _ := execution.Descriptor("workspace_read")
	allowed := s.profileActorCapabilityAllowed(s.agentActor(r), d) && restrictionAllowsDescriptor(e.Restrictions, d) && (e.Restrictions.Execution == nil || e.Restrictions.Execution.Read)
	if e.ProfileID != "" {
		profile, err := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), e.ProfileID, e.ProfileRevision)
		if err != nil {
			return nil, err
		}
		allowed = allowed && (profile.Settings.Execution == nil || profile.Settings.Execution.Read)
	}
	if !allowed {
		e.Folder.Reason = "Conversation or profile restrictions exclude the selected folder."
		return nil, nil
	}
	if s.executionHosts == nil {
		return nil, nil
	}
	resource, err := s.executionHosts.Resolve(r.Context(), s.agentActor(r), s.workspaceID, *ref)
	if err != nil || !resource.Permissions.Read {
		return nil, nil
	}
	e.Folder.Available, e.Folder.Reason = true, "The selected folder is connected. Each operation still requires live authorization."
	return &resource, nil
}

// Resolve saved context afresh for each attempt. Expired optional access must
// neither become authority nor prevent an unrelated text-only conversation.
func (s *Server) attachConversationFolder(r *http.Request, c *agents.Conversation, e *agents.EffectiveConversationPreferences, config *agents.AgentConfig) error {
	resource, err := s.conversationFolderResource(r, c, e)
	if err != nil {
		return err
	}
	if e.Folder != nil && !e.Folder.Available {
		config.SystemPrompt += "\nWorkspace limitation: " + e.Folder.Reason
	}
	if config.TextOnlyInference {
		return nil
	}
	if config.AllowedCapabilities == nil {
		config.AllowedCapabilities = map[string]capabilities.Descriptor{}
	}
	if resource != nil {
		ref := resource.Reference
		config.Execution, config.ExecutionWorkspace = &ref, resource
		config.InputWriteVersion = execution.InputWriteVersion
		for _, tool := range executionTools() {
			d, _ := executionDescriptor(tool.Function.Name)
			if s.profileActorCapabilityAllowed(s.agentActor(r), d) && restrictionAllowsDescriptor(e.Restrictions, d) && executionPermitted(d.Name, resource.Permissions) && (config.ExecutionPolicy == nil || executionPermitted(d.Name, *config.ExecutionPolicy)) {
				config.AllowedCapabilities[d.Name] = d
			}
		}
		config.SystemPrompt += "\nThe conversation's locally approved folder is attached. Use workspace_* tools for its files; never substitute service files or translate paths between hosts. Read and preserve exact bytes, and verify actual results."
	}
	d := execution.AccessRequestDescriptor()
	if s.config != nil && s.config.EnableExecutionPreview && s.executionHosts != nil && s.profileActorCapabilityAllowed(s.agentActor(r), d) && restrictionAllowsDescriptor(e.Restrictions, d) && (config.ExecutionPolicy == nil || config.ExecutionPolicy.Read) {
		config.ExecutionRequestVersion = execution.AccessRequestVersion
		config.AllowedCapabilities[d.Name] = d
	}
	return nil
}
