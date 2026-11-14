package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

// Omission inherits a profile selection. An explicitly empty list disables
// that resource category. Restrictions can only narrow the admitted profile.
type agentResourceRestrictions = agents.ResourceRestrictions

// An explicit field can change the user's restriction. Omission inherits the
// last accepted conversation setting, including across profile revisions.
func inheritConversationRestrictions(previous agents.ResourceRestrictions, requested *agentResourceRestrictions, profile *agents.AgentProfile) *agentResourceRestrictions {
	result := previous
	if requested != nil {
		if requested.Tools != nil {
			result.Tools = requested.Tools
		}
		if requested.Knowledge != nil {
			result.Knowledge = requested.Knowledge
		}
		if requested.Computer != nil {
			result.Computer = requested.Computer
		}
		if requested.Execution != nil {
			result.Execution = requested.Execution
		}
	}
	// A new profile may have fewer resources. Inherited selections narrow to
	// that ceiling; explicit invalid selections still fail normal admission.
	if previous.Tools != nil && (requested == nil || requested.Tools == nil) {
		selected := []agents.ProfileToolReference{}
		for _, ref := range *previous.Tools {
			if profileContainsTool(profile.Settings.Tools, ref) {
				selected = append(selected, ref)
			}
		}
		result.Tools = &selected
	}
	if previous.Knowledge != nil && (requested == nil || requested.Knowledge == nil) {
		selected := []string{}
		for _, id := range *previous.Knowledge {
			for _, available := range profile.Settings.Knowledge {
				if id == available {
					selected = append(selected, id)
					break
				}
			}
		}
		result.Knowledge = &selected
	}
	if previous.Computer != nil && (requested == nil || requested.Computer == nil) {
		selected := *previous.Computer && profile.Settings.Computer
		result.Computer = &selected
	}
	if previous.Execution != nil && profile.Settings.Execution != nil && (requested == nil || requested.Execution == nil) {
		selected := *previous.Execution
		selected.Read = selected.Read && profile.Settings.Execution.Read
		selected.Write = selected.Write && profile.Settings.Execution.Write
		selected.Processes = selected.Processes && profile.Settings.Execution.Processes
		result.Execution = &selected
	}
	return &result
}

func profileContainsTool(refs []agents.ProfileToolReference, target agents.ProfileToolReference) bool {
	for _, ref := range refs {
		if ref == target {
			return true
		}
	}
	return false
}

func intersectChildRestrictions(parent, child *agents.AgentProfile) *agentResourceRestrictions {
	tools := []agents.ProfileToolReference{}
	knowledge := []string{}
	for _, ref := range child.Settings.Tools {
		if profileContainsTool(parent.Settings.Tools, ref) {
			tools = append(tools, ref)
		}
	}
	for _, id := range child.Settings.Knowledge {
		for _, parentID := range parent.Settings.Knowledge {
			if id == parentID {
				knowledge = append(knowledge, id)
				break
			}
		}
	}
	return &agentResourceRestrictions{Tools: &tools, Knowledge: &knowledge}
}

// Restrict the readiness view before resolving unavailable resources. Keep the
// original immutable profile in the admitted config; this view grants nothing.
func restrictedProfileView(profile *agents.AgentProfile, restrictions *agentResourceRestrictions) (*agents.AgentProfile, bool) {
	view := *profile
	if restrictions == nil {
		return &view, true
	}
	if restrictions.Tools != nil {
		allowed := map[agents.ProfileToolReference]bool{}
		for _, ref := range profile.Settings.Tools {
			allowed[ref] = true
		}
		seen := map[agents.ProfileToolReference]bool{}
		for _, ref := range *restrictions.Tools {
			if !allowed[ref] || seen[ref] {
				return nil, false
			}
			seen[ref] = true
		}
		view.Settings.Tools = append([]agents.ProfileToolReference{}, (*restrictions.Tools)...)
	}
	if restrictions.Knowledge != nil {
		allowed := map[string]bool{}
		for _, id := range profile.Settings.Knowledge {
			allowed[id] = true
		}
		seen := map[string]bool{}
		for _, id := range *restrictions.Knowledge {
			if !allowed[id] || seen[id] {
				return nil, false
			}
			seen[id] = true
		}
		view.Settings.Knowledge = append([]string{}, (*restrictions.Knowledge)...)
	}
	if restrictions.Computer != nil {
		if *restrictions.Computer && !profile.Settings.Computer {
			return nil, false
		}
		view.Settings.Computer = *restrictions.Computer
	}
	return &view, true
}

func restrictProfileConfig(w http.ResponseWriter, restrictions *agentResourceRestrictions, config *agents.AgentConfig) bool {
	if config.Profile != nil && config.Profile.Settings.Execution != nil {
		limit := *config.Profile.Settings.Execution
		config.ExecutionPolicy = &limit
	}
	if restrictions == nil {
		return true
	}
	deny := func() bool {
		writeJobError(w, 403, "profile_resource_denied", "Selected resources must be a subset of this agent profile.", false)
		return false
	}
	if restrictions.Execution != nil {
		selected := *restrictions.Execution
		if limit := config.ExecutionPolicy; limit != nil && (selected.Read && !limit.Read || selected.Write && !limit.Write || selected.Processes && !limit.Processes) {
			return deny()
		}
		config.ExecutionPolicy = &selected
	}
	if restrictions.Computer != nil {
		if *restrictions.Computer && !config.Profile.Settings.Computer {
			return deny()
		}
		config.RestrictComputer = !*restrictions.Computer
		if config.RestrictComputer && (config.RequestedComputerScope != "" || len(config.NativeExpectations) > 0) {
			return deny()
		}
	}
	if restrictions.Tools != nil {
		allowed := map[string]capabilities.Descriptor{}
		for _, ref := range *restrictions.Tools {
			d, ok := config.AllowedCapabilities[ref.Tool]
			if !ok || d.Source != ref.Provider || d.Revision != ref.SchemaRevision || d.Location != ref.Location {
				return deny()
			}
			if _, duplicate := allowed[ref.Tool]; duplicate {
				return deny()
			}
			allowed[ref.Tool] = d
		}
		config.AllowedCapabilities = allowed
	}
	if restrictions.Knowledge != nil {
		seen := map[string]bool{}
		for _, id := range *restrictions.Knowledge {
			found := false
			for _, selected := range config.Profile.Settings.Knowledge {
				if selected == id {
					found = true
					break
				}
			}
			if !found || seen[id] {
				return deny()
			}
			seen[id] = true
		}
		selected := append([]string{}, (*restrictions.Knowledge)...)
		config.KnowledgeSelection = &selected
	}
	return true
}

func selectedProfileKnowledge(config agents.AgentConfig) []string {
	if config.KnowledgeSelection != nil {
		return *config.KnowledgeSelection
	}
	if config.Profile == nil {
		return nil
	}
	return config.Profile.Settings.Knowledge
}
