package server

import (
	"context"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// registryToolAllowed is shared by discovery and dispatch. A companion session
// adds a provider to task-first work; it never broadens legacy computer-only work.
func (b *browserRunTools) registryToolAllowed(task *agents.Task, name string) bool {
	if task.Config.ComputerSession != "" && !task.Config.TaskFirst {
		return false
	}
	d, ok := b.RunTools.Capability(name)
	if !ok {
		return false
	}
	if task.Config.TaskFirst && (name == "shell" || d.Kind == capabilities.Execute) {
		// Remote metadata cannot lower risk. MCP tools still pass their own
		// registry policy/approval; local scripts cannot hide behind another name.
		return name != "shell" && d.Namespace == "mcp" && strings.HasPrefix(d.Source, "mcp:")
	}
	return true
}

// ToolsForTask is retained for existing internal clients. The durable runner
// uses the context-aware form so canceled work does not perform discovery.
func (b *browserRunTools) ToolsForTask(task *agents.Task) []api.Tool {
	return b.ToolsForTaskContext(context.Background(), task)
}

func (b *browserRunTools) ToolsForTaskContext(ctx context.Context, task *agents.Task) []api.Tool {
	if ctx.Err() != nil {
		return nil
	}
	var tools []api.Tool
	if task.Config.ComputerSession == "" || task.Config.TaskFirst {
		registryTools := b.RunTools.GetTools()
		if scoped, ok := b.RunTools.(interface {
			ToolsForExecution(context.Context, agents.ToolExecution) []api.Tool
		}); ok {
			registryTools = scoped.ToolsForExecution(ctx, agents.ToolExecution{Actor: task.Actor, RunID: task.ID})
		}
		for _, tool := range registryTools {
			name := tool.Function.Name
			if _, reserved := browserDescriptor(name); reserved || name == computerAccessToolName || name == taskArtifactToolName || !b.registryToolAllowed(task, name) {
				continue
			}
			if task.Config.TaskFirst {
				switch name {
				case "read_file", "list_files", "write_file":
					tool.Function.Description += " Operates in the SERVICE filesystem only, not the user's desktop. For the user's files, request computer access to their file manager instead."
				case "current_time":
					tool.Function.Description += " Returns the SERVICE clock with its timezone, not the desktop's local timezone."
				}
			}
			tools = append(tools, tool)
		}
	}
	if task.Config.ComputerSession != "" {
		if nativeComputerDriver(task.Config.ComputerDriver) {
			tools = append(tools, nativeComputerTools()...)
			if task.Config.ComputerScope == "desktop" {
				tools = append(tools, desktopComputerTools()...)
			}
		} else {
			tools = append(tools, browserToolsForVision(b.server.registry != nil && b.server.computerVisionReady(task.Model))...)
		}
	}
	if task.Config.TaskFirst && task.ParentID == "" {
		if task.Config.ComputerScope != "desktop" {
			tools = append(tools, computerAccessTool())
		}
		if b.server.artifactStore != nil {
			tools = append(tools, taskArtifactTool())
		}
	}
	return tools
}
