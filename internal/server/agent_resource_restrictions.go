package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Omission inherits a profile selection. An explicitly empty list disables
// that resource category. Restrictions can only narrow the admitted profile.
type agentResourceRestrictions struct {
	Tools     *[]agents.ProfileToolReference `json:"tools,omitempty"`
	Knowledge *[]string                      `json:"knowledge,omitempty"`
	Computer  *bool                          `json:"computer,omitempty"`
	Execution *execution.Permissions         `json:"execution,omitempty"`
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
	return config.Profile.Settings.Knowledge
}
