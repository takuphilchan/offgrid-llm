package agents

import "github.com/takuphilchan/offgrid-llm/pkg/api"

// Omitted fields inherit; explicit empty lists disable that resource category.
// These are restrictions, not host grants or credentials.
type ResourceRestrictions = api.ResourceRestrictions

// Preserve only settings evidenced by a versioned accepted conversation turn.
// Older browser-only choices remain unknown until explicitly selected again.
type ConversationConfiguration struct {
	Version      int                  `json:"version"`
	Profile      *AgentProfile        `json:"profile"`
	Model        string               `json:"model"`
	Restrictions ResourceRestrictions `json:"restrictions"`
}

// The accepted Chat choice is immutable provenance. A later effective model may
// differ only through the recorded adaptive chain, not a rewritten preference.
func validAcceptedConversationModel(task *Task, accepted string) bool {
	cp, selection := task.Checkpoint, task.Config.ModelSelection
	if cp == nil || len(cp.ModelSwitches) == 0 {
		return task.Model == accepted
	}
	return selection != nil && selection.Mode == "adaptive" && cp.ModelSwitches[0].From == accepted && validSwitchSelection(task)
}
