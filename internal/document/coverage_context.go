package document

import "encoding/json"

// Allow page-level warnings on the bounded 1,000-page extraction contract.
// Model context uses a separate summary; storage never silently drops warnings.
const MaxCoverageBytes = 2 << 20

// ContextJSON bounds repeated warning prose without changing the retained
// receipt. Counts and review-unit identities remain explicit. The complete
// warnings are available from the authorized source/preview APIs.
func (c *Coverage) ContextJSON() json.RawMessage {
	if c == nil {
		return nil
	}
	copy := *c
	copy.Warnings = nil
	seen := map[string]bool{}
	sampled := false
	for _, warning := range c.Warnings {
		key := warning.Code + "\x00" + warning.Message
		if seen[key] || len(copy.Warnings) == 8 {
			sampled = true
			continue
		}
		seen[key] = true
		text := []rune(warning.Message)
		if len(text) > 512 {
			warning.Message = string(text[:512]) + "…"
			sampled = true
		}
		copy.Warnings = append(copy.Warnings, warning)
	}
	data, _ := json.Marshal(struct {
		Coverage
		WarningCount   int  `json:"warning_count"`
		WarningSampled bool `json:"warning_summary"`
	}{copy, len(c.Warnings), sampled})
	return data
}
