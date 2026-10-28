package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func (s *Server) checkProfileSkills(profile *agents.AgentProfile, issue func(string, string, string, string)) {
	for _, ref := range profile.Settings.Skills {
		item, _, err := s.agentManager.GetSkill(profile.Workspace, profile.Actor, ref)
		if err != nil || item.Draft {
			issue("skill", ref.ID, "skill_unavailable", "The attached skill revision is missing, corrupt, a draft or not owned by this account. Review the attachment; no replacement was loaded.")
		}
	}
}

func (s *Server) pinProfileContext(config *agents.AgentConfig) error {
	profile := config.Profile
	if profile == nil {
		return nil
	}
	config.SkillMetadata = nil
	for _, ref := range profile.Settings.Skills {
		item, _, err := s.agentManager.GetSkill(profile.Workspace, profile.Actor, ref)
		if err != nil {
			return err
		}
		if item.Draft {
			return agents.ErrContextNotFound
		}
		config.SkillMetadata = append(config.SkillMetadata, *item)
	}
	consent, err := s.agentManager.GetMemoryConsent(profile.Workspace, profile.Actor, profile.ID)
	if err != nil {
		return err
	}
	config.MemoryConsentRevision = 0
	if consent.Enabled {
		config.MemoryConsentRevision = consent.Revision
	}
	if config.AllowedCapabilities == nil {
		config.AllowedCapabilities = map[string]capabilities.Descriptor{}
	}
	for _, tool := range agentcontext.Tools() {
		name := tool.Function.Name
		if name == "skill_load" && len(config.SkillMetadata) == 0 {
			continue
		}
		if (name == "memory_read" || name == "memory_write") && config.MemoryConsentRevision == 0 {
			continue
		}
		d, _ := agentcontext.Descriptor(name)
		config.AllowedCapabilities[name] = d
	}
	return nil
}
