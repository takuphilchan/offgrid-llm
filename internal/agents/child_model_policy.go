package agents

import (
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// Conversation workers keep the current qualified model unless a selected
// immutable profile explicitly enables a local pool. Legacy direct profile
// delegation retains its separately selected child model. No name/role heuristic
// changes models, and this function never loads a runtime or grants tools.
func configureChildInference(config *AgentConfig, parent *Task, selected *AgentConfig) (string, error) {
	source, model := &parent.Config, parent.Model
	if selected != nil {
		if selected.Profile.Settings.ModelPolicy != nil && selected.ModelSelection == nil {
			return "", errors.New("The saved child profile has no admitted model-policy evidence. Submit new work to resolve its current qualification.")
		}
		if parent.WorkID == "" || selected.ModelSelection != nil {
			source, model = selected, selected.Profile.Settings.Model
		}
	}
	if source.ContextWindow > 0 {
		config.ContextWindow = source.ContextWindow
	}
	if source.ToolAdapter != nil {
		pin := *source.ToolAdapter
		config.ToolAdapter = &pin
	}
	if source.ModelSelection != nil {
		encoded, err := json.Marshal(source.ModelSelection)
		if err != nil {
			return "", err
		}
		var selection ModelPolicySelection
		if err = json.Unmarshal(encoded, &selection); err != nil {
			return "", err
		}
		if selection.Version != 1 || selection.Selected != model || selection.Mode != "guided" && selection.Mode != "adaptive" {
			return "", errors.New("The selected child's model policy does not match its model.")
		}
		matched := false
		for _, candidate := range selection.Candidates {
			if candidate.Model == model && candidate.ContextWindow == config.ContextWindow && sameModelAdapter(candidate.ToolAdapter, config.ToolAdapter) {
				matched = true
			}
		}
		if !matched {
			return "", errors.New("The selected child model lacks matching context and adapter evidence.")
		}
		if source == &parent.Config {
			selection.Reason = "inherited_qualified_model"
		}
		config.ModelSelection = &selection
	}
	if config.ToolAdapter != nil {
		if err := ValidateAgentToolScope(config.ToolAdapter); err != nil {
			return "", err
		}
		config.TaskFirst = true
		var err error
		config.SystemPrompt, err = config.initialSystemPrompt()
		if err != nil {
			return "", err
		}
	}
	config.SystemPrompt += contextResourcePrompt(*config)
	return model, nil
}

func sameModelAdapter(a, b *inference.AgentToolPin) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
