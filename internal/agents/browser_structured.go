package agents

import (
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func structuredBrowserRead(name string, descriptor capabilities.Descriptor) bool {
	switch name {
	case "browser_search", "browser_frames", "browser_frame_select":
		return descriptor.Name == name && capabilities.TrustedBrowserRead(descriptor)
	}
	return false
}

// Frame inventory is discovery, not an observation of its page contents.
func structuredBrowserObservation(step Step, tools interface {
	Capability(string) (capabilities.Descriptor, bool)
}) bool {
	if step.Type != "action" || step.ToolName != "browser_search" && step.ToolName != "browser_frame_select" {
		return false
	}
	d, known := tools.Capability(step.ToolName)
	var result struct {
		Observation string `json:"observation_id"`
	}
	return known && structuredBrowserRead(step.ToolName, d) && json.Unmarshal([]byte(step.ToolResult), &result) == nil && strings.TrimSpace(result.Observation) != ""
}
