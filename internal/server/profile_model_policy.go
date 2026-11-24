package server

import (
	"context"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

// This is admission, not runtime selection: it loads no model and changes no
// residency. Candidates must pass the ordinary scheduler again when used.
func (s *Server) resolveProfileModelPolicy(ctx context.Context, settings agents.ProfileSettings, selected string, window func(string) int) (*agents.ModelPolicySelection, *profileResourceIssue) {
	policy := settings.ModelPolicy
	if policy == nil {
		return nil, nil
	}
	issue := func(ref, code, message string) (*agents.ModelPolicySelection, *profileResourceIssue) {
		return nil, &profileResourceIssue{Kind: "model_policy", Reference: ref, Code: code, Message: message}
	}
	if policy.Validate(settings.Model) != nil {
		return issue(settings.Model, "model_policy_invalid", "Use a version 1 guided or adaptive local policy with at most two distinct fallback models.")
	}
	if selected == "" {
		selected = settings.Model
	}
	if !policy.Allows(settings.Model, selected) {
		return issue(selected, "profile_model_mismatch", "Select the primary model or an explicitly configured local fallback.")
	}
	selection := &agents.ModelPolicySelection{Version: 1, Mode: policy.Mode, Primary: settings.Model, Selected: selected, Reason: "primary_model"}
	if selected != settings.Model {
		selection.Reason = "explicit_qualified_fallback"
	}
	ids := append([]string{settings.Model}, policy.FallbackModels...)
	for i, id := range ids {
		if s.registry == nil {
			return issue(id, "model_unavailable", "Model storage is unavailable.")
		}
		if _, err := s.localChatMetadata(id); err != nil {
			return issue(id, "local_model_unavailable", "Select an installed local chat model. No model or endpoint was acquired.")
		}
		candidate := agents.ModelCandidate{Model: id, ContextWindow: window(id)}
		pin, err := s.agentToolPolicy.Select(ctx, id, candidate.ContextWindow)
		if err != nil {
			return issue(id, "model_measurement_unavailable", "Current model qualification could not be verified.")
		}
		// Existing primary models retain their constrained path in guided mode.
		// A fallback or adaptive pool needs actual matching workflow evidence.
		if pin == nil && (i != 0 || policy.Mode == "adaptive") {
			return issue(id, "local_model_unqualified", "This local model has no matching workflow qualification for the planned context and runtime.")
		}
		candidate.ToolAdapter = pin
		selection.Candidates = append(selection.Candidates, candidate)
	}
	if id := policy.PerceptionModel; id != "" {
		metadata, err := s.localChatMetadata(id)
		if err != nil || metadata.ProjectorPath == "" {
			return issue(id, "perception_model_unavailable", "Select an installed local vision model with its projector.")
		}
		candidate := &agents.ModelCandidate{Model: id, ContextWindow: window(id)}
		pin, err := s.agentToolPolicy.SelectPerception(ctx, id, candidate.ContextWindow)
		if err != nil || pin == nil {
			return issue(id, "perception_model_unqualified", "Perception requires matching full workflow evidence with measured projector and preprocessing identities.")
		}
		candidate.ToolAdapter, selection.Perception = pin, candidate
	}
	return selection, nil
}

func (s *Server) pinJobModelPolicy(w http.ResponseWriter, r *http.Request, req *jobSubmitRequest, config *agents.AgentConfig) bool {
	if config.Profile == nil || config.Profile.Settings.ModelPolicy == nil {
		return true
	}
	settings := config.Profile.Settings
	if req.ConversationModelOverride && !settings.ModelPolicy.Allows(settings.Model, req.Model) {
		// A separately explicit conversation choice keeps its existing semantics;
		// it does not inherit a pool approved for a different primary model.
		return true
	}
	window := func(id string) int {
		if id == req.Model {
			return config.ContextWindow
		}
		value := s.modelContextWindow(id)
		if req.ResponseProfile != "" {
			value = min(value, s.chatContextWindowForModel(req.ResponseProfile, id))
		}
		return value
	}
	selection, issue := s.resolveProfileModelPolicy(r.Context(), settings, req.Model, window)
	if issue != nil {
		writeJobError(w, 422, issue.Code, issue.Message, false)
		return false
	}
	// The selected adapter must be precisely the measured candidate, including
	// its capability scope; an approved model cannot widen tool qualification.
	for _, candidate := range selection.Candidates {
		if candidate.Model == req.Model {
			config.ToolAdapter = candidate.ToolAdapter
			break
		}
	}
	config.ModelSelection = selection
	return true
}

func (s *Server) pinDelegateModelPolicy(w http.ResponseWriter, r *http.Request, profile *effectiveAgentProfile, window func(string) int, config *agents.AgentConfig) bool {
	model := profile.Profile.Settings.Model
	config.ContextWindow = window(model)
	config.ModelSelection = profile.ModelSelection
	if config.ModelSelection != nil {
		for _, candidate := range config.ModelSelection.Candidates {
			if candidate.Model == model {
				config.ContextWindow, config.ToolAdapter = candidate.ContextWindow, candidate.ToolAdapter
				return true
			}
		}
		writeJobError(w, 422, "delegate_model_policy_invalid", "The selected child model is absent from its qualified policy.", false)
		return false
	}
	pin, err := s.agentToolPolicy.Select(r.Context(), model, config.ContextWindow)
	if err != nil {
		writeJobError(w, 422, "delegate_adapter_unavailable", "A child model's tool qualification could not be verified. No job was started.", false)
		return false
	}
	config.ToolAdapter = pin
	return true
}
