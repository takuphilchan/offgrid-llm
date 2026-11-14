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
