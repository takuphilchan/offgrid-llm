package agents

import (
	"context"
	"errors"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func companionCapability(d capabilities.Descriptor) bool {
	return d.Kind == capabilities.Computer && d.Namespace == "computer" && d.Source == "local-companion"
}

func workspaceArtifactCapability(name string, d capabilities.Descriptor) bool {
	return name == "task_save_artifact" && d.Name == name && d.Kind == capabilities.Write && d.Namespace == "workspace" && d.Source == "workspace"
}

// ToolsForExecution discovers enabled tools for an authenticated actor/run. It
// does not approve an action: execution repeats policy with the actual arguments.
// Approval-required tools remain discoverable so the normal approval flow works.
func (r *ToolRegistry) ToolsForExecution(ctx context.Context, execution ToolExecution) []api.Tool {
	type candidate struct {
		tool       api.Tool
		descriptor capabilities.Descriptor
	}
	r.mu.RLock()
	broker := r.broker
	candidates := make([]candidate, 0, len(r.tools))
	for name, tool := range r.tools {
		if !r.disabledTools[name] {
			candidates = append(candidates, candidate{tool, r.capabilityLocked(name)})
		}
	}
	r.mu.RUnlock()

	tools := make([]api.Tool, 0, len(candidates))
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return nil
		}
		if broker != nil {
			_, err := broker.Authorize(ctx, capabilities.Request{Actor: execution.Actor, RunID: execution.RunID, Capability: candidate.descriptor})
			if err != nil && !errors.Is(err, capabilities.ErrApprovalRequired) {
				continue
			}
		}
		tools = append(tools, candidate.tool)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Function.Name < tools[j].Function.Name })
	return tools
}
