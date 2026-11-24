package agents

import (
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// ProfileModelPolicy is an immutable preference ceiling, not model acquisition
// or execution authority. An omitted policy preserves historical profile bytes.
type ProfileModelPolicy struct {
	Version         int      `json:"version"`
	Mode            string   `json:"mode"`
	FallbackModels  []string `json:"fallback_models,omitempty"`
	PerceptionModel string   `json:"perception_model,omitempty"`
}

func (p *ProfileModelPolicy) Validate(primary string) error {
	if p == nil {
		return nil
	}
	if p.Version != 1 || (p.Mode != "guided" && p.Mode != "adaptive") || len(p.FallbackModels) > 2 {
		return ErrInvalidProfile
	}
	seen := map[string]bool{primary: true}
	for _, id := range p.FallbackModels {
		if !localModelReference(id) || seen[id] {
			return ErrInvalidProfile
		}
		seen[id] = true
	}
	if !localModelReference(primary) || (p.PerceptionModel != "" && !localModelReference(p.PerceptionModel)) {
		return ErrInvalidProfile
	}
	return nil
}

func localModelReference(id string) bool {
	// Endpoint routes are authorized independently and are never local pools,
	// even when the selected endpoint happens to use a loopback address.
	return validProfileText(id, 256, true) && !strings.ContainsAny(id, "\r\n") && !strings.HasPrefix(id, "endpoint:")
}

func (p *ProfileModelPolicy) Allows(primary, selected string) bool {
	return selected == primary || p != nil && slices.Contains(p.FallbackModels, selected)
}

// ModelCandidate pins reviewed compatibility evidence at admission. The
// runtime still revalidates the selected pin while holding its inference lease.
type ModelCandidate struct {
	Model         string                  `json:"model"`
	ContextWindow int                     `json:"context_window"`
	ToolAdapter   *inference.AgentToolPin `json:"tool_adapter,omitempty"`
}

type ModelPolicySelection struct {
	Version    int              `json:"version"`
	Mode       string           `json:"mode"`
	Primary    string           `json:"primary"`
	Selected   string           `json:"selected"`
	Reason     string           `json:"reason"`
	Candidates []ModelCandidate `json:"candidates"`
	Perception *ModelCandidate  `json:"perception,omitempty"`
}
