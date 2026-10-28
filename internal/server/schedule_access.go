package server

import (
	"context"
	"net/http"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type scheduleBlocked string

func (e scheduleBlocked) Error() string { return string(e) }

func (s *Server) scheduleActorPermission(actor string, permission users.Permission) bool {
	if s.config == nil || !s.config.RequireAuth {
		return actor == "local-admin"
	}
	return s.userStore != nil && s.userStore.ActorHasPermission(actor, permission)
}

// This validates the pinned identity, never resolves a newer profile/model or
// restores host/computer access. Each provider still checks its own policy.
func (s *Server) validateScheduleResources(ctx context.Context, actor string, config agents.AgentConfig, child bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.validateScheduleProfile(actor, config); err != nil {
		return err
	}
	profile := config.Profile
	var err error
	if config.ComputerSession != "" || config.RequestedComputerScope != "" || config.ComputerScope != "" || config.ComputerAccessOnDemand || !config.RestrictComputer {
		return scheduleBlocked("scheduled_computer_access_unavailable")
	}
	if _, native, err := s.selectNativeChat(ctx, profile.Settings.Model); native {
		if err != nil || !config.TextOnlyInference || child {
			return scheduleBlocked("model_runtime_unavailable")
		}
	} else if config.TextOnlyInference || s.registry == nil {
		return scheduleBlocked("model_runtime_changed")
	} else {
		model, lookupErr := s.registry.GetChatModel(profile.Settings.Model)
		if lookupErr != nil {
			return scheduleBlocked("model_unavailable")
		}
		info, statErr := os.Stat(model.Path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return scheduleBlocked("model_unavailable")
		}
	}
	for _, ref := range profile.Settings.Skills {
		item, _, err := s.agentManager.GetSkill(s.workspaceID, actor, ref)
		if err != nil || item.Draft {
			return scheduleBlocked("skill_revision_unavailable")
		}
	}
	if config.MemoryConsentRevision > 0 {
		consent, err := s.agentManager.GetMemoryConsent(s.workspaceID, actor, profile.ID)
		if err != nil || !consent.Enabled || consent.Revision != config.MemoryConsentRevision {
			return scheduleBlocked("memory_consent_changed")
		}
	}
	if len(selectedProfileKnowledge(config)) > 0 {
		if !s.scheduleActorPermission(actor, users.PermissionRAG) || s.ragEngine == nil || !s.ragEngine.IsEnabled() || s.ragEngine.StorageError() != nil {
			return scheduleBlocked("knowledge_unavailable")
		}
		for _, id := range selectedProfileKnowledge(config) {
			if s.ragEngine.GetDocument(id) == nil {
				return scheduleBlocked("knowledge_document_unavailable")
			}
		}
	}
	catalog := map[string]capabilities.Descriptor{}
	if s.toolRegistry != nil {
		for _, tool := range s.toolRegistry.ToolsForExecution(ctx, agents.ToolExecution{Actor: actor}) {
			if d, ok := s.toolRegistry.Capability(tool.Function.Name); ok {
				catalog[d.Name] = d
			}
		}
	}
	for name, pinned := range config.AllowedCapabilities {
		if _, ok := contextDescriptor(name); ok {
			continue
		}
		if _, ok := executionDescriptor(name); ok {
			continue
		}
		if name == taskArtifactToolName {
			continue
		}
		if current, ok := catalog[name]; !ok || current != pinned || !s.profileActorCapabilityAllowed(actor, current) {
			return scheduleBlocked("tool_policy_or_schema_changed")
		}
	}
	if config.Execution != nil {
		if child || s.executionHosts == nil {
			return scheduleBlocked("execution_host_unavailable")
		}
		resource, err := s.executionHosts.Resolve(ctx, actor, s.workspaceID, *config.Execution)
		if err != nil || !executionScopeMatches(&agents.Task{Config: config}, resource) {
			return scheduleBlocked("execution_host_unavailable")
		}
	}
	for _, delegate := range config.DelegateProfiles {
		if child {
			return scheduleBlocked("nested_delegation_unavailable")
		}
		if err = s.validateScheduleResources(ctx, actor, delegate, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) validateScheduleProfile(actor string, config agents.AgentConfig) error {
	if !s.scheduleActorPermission(actor, users.PermissionChat) {
		return scheduleBlocked("actor_permission_revoked")
	}
	profile := config.Profile
	if profile == nil || profile.Actor != actor || profile.Workspace != s.workspaceID {
		return scheduleBlocked("profile_scope_invalid")
	}
	stored, err := s.agentManager.GetProfile(s.workspaceID, actor, profile.ID, profile.Revision)
	if err != nil || stored.Archived || stored.Digest != profile.Digest {
		return scheduleBlocked("profile_revision_unavailable")
	}
	return nil
}

func (s *Server) scheduledTaskAccess(ctx context.Context, task *agents.Task, tool string) error {
	root, err := s.agentManager.ScheduleRootForTask(task)
	if err != nil {
		return capabilities.ErrDenied
	}
	if root == nil {
		return nil
	}
	if err := s.agentManager.ScheduleAllowsTask(root); err != nil {
		return &agents.ToolAuthorizationError{Code: "schedule_authority_changed", Message: "The schedule was disabled, revised or its saved occurrence changed. No new action was dispatched.", Cause: err}
	}
	if task.ParentID != "" && root.Status != agents.TaskChildren {
		return capabilities.ErrDenied
	}
	if tool == computerAccessToolName {
		return capabilities.ErrDenied
	}
	if _, computer := browserDescriptor(tool); computer {
		return capabilities.ErrDenied
	}
	// Do not disable independent branches just because an unrelated attachment
	// became unavailable. Providers recheck the requested tool/resource below.
	if err := s.validateScheduleProfile(root.Actor, root.Config); err != nil {
		return err
	}
	if err := s.validateScheduleProfile(task.Actor, task.Config); err != nil {
		return &agents.ToolAuthorizationError{Code: "schedule_resource_unavailable", Message: "A pinned schedule resource or permission is unavailable. Inspect the saved occurrence; no replacement resource was selected.", Cause: err}
	}
	return nil
}

func (s *Server) prepareScheduleKnowledge(ctx context.Context, prompt string, config *agents.AgentConfig) error {
	// The actor/resource checks above already authorize the exact document IDs.
	// This helper uses only cancellation from this request; no identity is forged
	// and no HTTP request is sent or synthetic credential installed.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://owned.invalid/knowledge-context", nil)
	if err != nil {
		return err
	}
	writer := newCaptureResponseWriter()
	if !s.attachProfileKnowledge(writer, request, prompt, config) {
		return scheduleBlocked("knowledge_retrieval_unavailable")
	}
	children := make(map[string]agents.AgentConfig, len(config.DelegateProfiles))
	for id, child := range config.DelegateProfiles {
		if err = s.prepareScheduleKnowledge(ctx, prompt, &child); err != nil {
			return err
		}
		children[id] = child
	}
	config.DelegateProfiles = children
	return nil
}

func scheduleFailureCode(err error) string {
	if code, ok := err.(scheduleBlocked); ok {
		return string(code)
	}
	return "schedule_admission_unavailable"
}
