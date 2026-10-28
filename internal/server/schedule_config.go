package server

import (
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

type scheduleWriteRequest struct {
	ExpectedRevision int                     `json:"expected_revision"`
	Settings         agents.ScheduleSettings `json:"settings"`
	ProfileID        string                  `json:"profile_id"`
	ProfileRevision  int                     `json:"profile_revision"`
	Execution        *execution.Reference    `json:"execution,omitempty"`
	MaxIterations    int                     `json:"max_iterations,omitempty"`
}

func scheduledConfiguration(config *agents.AgentConfig) {
	config.RestrictComputer = true
	config.ComputerAccessOnDemand = false
	config.RequestedComputerScope = ""
	config.RequestedComputerApprovalMode = ""
	config.SystemPrompt += "\nThis is an explicitly reviewed scheduled task. Use only its pinned profile and currently authorized resources. No desktop or browser-control consent is available; do not request it. Report blocked or partial outcomes honestly, never substitute another host/model or replay an uncertain action."
	children := make(map[string]agents.AgentConfig, len(config.DelegateProfiles))
	for id, child := range config.DelegateProfiles {
		scheduledConfiguration(&child)
		children[id] = child
	}
	config.DelegateProfiles = children
}

func (s *Server) resolveScheduleConfiguration(w http.ResponseWriter, r *http.Request, request scheduleWriteRequest) (agents.AgentConfig, bool) {
	if request.ProfileID == "" || request.ProfileRevision < 1 || request.ExpectedRevision < 0 {
		writeJobError(w, 400, "invalid_schedule", "Select an exact owned profile revision and an explicit calendar policy.", false)
		return agents.AgentConfig{}, false
	}
	computer := false
	if _, err := request.Settings.Calendar.Canonical(); err != nil {
		writeScheduleError(w, err)
		return agents.AgentConfig{}, false
	}
	job := jobSubmitRequest{DeferKnowledge: true, Prompt: request.Settings.Prompt, ProfileID: request.ProfileID, ProfileRevision: request.ProfileRevision, Execution: request.Execution, MaxIterations: request.MaxIterations, Restrictions: &agentResourceRestrictions{Computer: &computer}}
	config, valid := s.resolveJobConfig(w, r, &job)
	if !valid {
		return config, false
	}
	scheduledConfiguration(&config)
	if err := s.validateScheduleResources(r.Context(), s.agentActor(r), config, false); err != nil {
		writeJobError(w, 422, "schedule_resource_unavailable", "A pinned schedule resource is unavailable: "+scheduleFailureCode(err)+". Review it before saving.", false)
		return config, false
	}
	return config, true
}
