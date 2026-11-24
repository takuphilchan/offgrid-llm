package server

import (
	"context"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Pin only local capability metadata. No host reference, provider invocation,
// resource contents or approval is retained in this separate discovery ceiling.
func (s *Server) pinConversationDiscovery(config *agents.AgentConfig) {
	if config.TextOnlyInference {
		return
	}
	scope := &agents.ConversationDiscoveryScope{Version: 1, Capabilities: map[string]capabilities.Descriptor{}}
	add := func(d capabilities.Descriptor) {
		if identity, valid := agents.ConversationMetadataIdentity(d); valid {
			scope.Capabilities[d.Name] = identity
		}
	}
	adapter := &browserRunTools{RunTools: s.toolRegistry, server: s}
	view := &agents.Task{Config: *config}
	for name, d := range config.AllowedCapabilities {
		if !agents.ProfileAllowsCapability(*config, name, d) {
			continue
		}
		if d.Namespace == "tools" || d.Namespace == "mcp" {
			if s.toolRegistry == nil || !adapter.registryToolAllowed(view, name) {
				continue
			}
		}
		add(d)
	}
	if s.browserHub != nil && !config.RestrictComputer && (config.Profile == nil || config.Profile.Settings.Computer) {
		add(computerAccessDescriptor())
	}
	if s.artifactStore != nil {
		add(taskArtifactDescriptor())
	}
	config.ConversationDiscovery = scope
}

// ConversationToolMetadata is deliberately distinct from ToolsForTaskContext:
// that executable discovery path may check an execution host or load a skill
// package. This path reads only local scope/catalog metadata and returns no schema.
func (b *browserRunTools) ConversationToolMetadata(ctx context.Context, task *agents.Task) []capabilities.Descriptor {
	if ctx.Err() != nil || task == nil || task.ParentID != "" || task.ConversationTurn == nil || task.Config.TextOnlyInference ||
		task.Config.ConversationDiscovery == nil || task.Config.ConversationDiscovery.Version != 1 || b.server == nil ||
		b.server.agentManager == nil || !b.server.isCanonicalConversationRun(task) || !b.server.scheduleActorPermission(task.Actor, users.PermissionChat) {
		return nil
	}
	s := b.server
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: task.Actor}
	conversation, err := s.agentManager.Conversation(scope, task.Origin.Conversation)
	if err != nil {
		return nil
	}
	var profile *agents.AgentProfile
	if conversation.Preferences.ProfileID != "" {
		profile, err = s.agentManager.GetProfile(scope.Workspace, scope.Actor, conversation.Preferences.ProfileID, conversation.Preferences.ProfileRevision)
		if err != nil || profile.Archived {
			return nil
		}
	}
	// Current enabled registry inventory applies the actor broker before ranking.
	registry := map[string]bool{}
	if provider, ok := b.RunTools.(interface {
		ToolsForExecution(context.Context, agents.ToolExecution) []api.Tool
	}); ok {
		for _, tool := range provider.ToolsForExecution(ctx, agents.ToolExecution{Actor: task.Actor, RunID: task.ID}) {
			registry[tool.Function.Name] = true
		}
	}
	result := []capabilities.Descriptor{}
	for name, pinned := range task.Config.ConversationDiscovery.Capabilities {
		if ctx.Err() != nil {
			return nil
		}
		d, exists := b.Capability(name)
		identity, valid := agents.ConversationMetadataIdentity(d)
		if !exists || !valid || identity != pinned || !s.profileActorCapabilityAllowed(task.Actor, d) {
			continue
		}
		restrictions := conversation.Preferences.Restrictions
		switch {
		case d == computerAccessDescriptor():
			if s.browserHub == nil || restrictions.Computer != nil && !*restrictions.Computer || profile != nil && !profile.Settings.Computer {
				continue
			}
		case d == taskArtifactDescriptor():
			if s.artifactStore == nil {
				continue
			}
		case d == execution.AccessRequestDescriptor():
			if s.config == nil || !s.config.EnableExecutionPreview || s.executionHosts == nil || !restrictionAllowsDescriptor(restrictions, d) || restrictions.Execution != nil && !restrictions.Execution.Read || profile != nil && profile.Settings.Execution != nil && !profile.Settings.Execution.Read {
				continue
			}
		case executionToolNamespace(name):
			if !restrictionAllowsDescriptor(restrictions, d) || restrictions.Execution != nil && !executionPermitted(name, *restrictions.Execution) || profile != nil && profile.Settings.Execution != nil && !executionPermitted(name, *profile.Settings.Execution) {
				continue
			}
		case d.Namespace == "context" && d.Source == "agent-context":
			if !restrictionAllowsDescriptor(restrictions, d) || !b.conversationContextMetadataAllowed(task, profile, name) {
				continue
			}
		default:
			if !registry[name] || !b.registryToolAllowed(task, name) || !restrictionAllowsDescriptor(restrictions, d) || profile != nil && !profileContainsTool(profile.Settings.Tools, agents.ProfileToolReference{Provider: d.Source, Tool: name, SchemaRevision: d.Revision, Location: d.Location}) {
				continue
			}
		}
		result = append(result, d)
	}
	return result
}

func (b *browserRunTools) conversationContextMetadataAllowed(task *agents.Task, profile *agents.AgentProfile, name string) bool {
	if task.Config.Profile != nil && (profile == nil || profile.ID != task.Config.Profile.ID || profile.Revision != task.Config.Profile.Revision) {
		return false
	}
	if strings.HasPrefix(name, "memory_") {
		if profile == nil || task.Config.MemoryConsentRevision <= 0 {
			return false
		}
		consent, err := b.server.agentManager.GetMemoryConsent(b.server.workspaceID, task.Actor, profile.ID)
		return err == nil && consent.Enabled && consent.Revision == task.Config.MemoryConsentRevision
	}
	if name != "skill_load" {
		return profile != nil
	}
	for _, retained := range task.Config.SkillMetadata {
		if !b.server.agentManager.SkillReferenceAvailable(b.server.workspaceID, task.Actor, retained.SkillReference) {
			continue
		}
		if profile == nil {
			return true
		}
		for _, selected := range profile.Settings.Skills {
			if selected == retained.SkillReference {
				return true
			}
		}
	}
	return false
}
