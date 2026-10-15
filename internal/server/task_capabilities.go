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
			nativeTools := nativeComputerTools()
			documentsAvailable := b.documentToolsAvailable(task)
			if documentsAvailable {
				state, _ := documentState(task)
				controlObserved := task.Config.ComputerScope != "desktop"
				for i, step := range task.Steps {
					if i >= task.Config.ComputerStepOffset && step.Type == "action" && step.ToolName == "computer_observe" {
						controlObserved = true
					}
					if i >= task.Config.ComputerStepOffset && step.Type == "action" && (step.ToolName == "computer_open_application" || step.ToolName == "computer_focus_window") {
						controlObserved = false
					}
				}
				// The document and legacy whole-control shapes must not be offered
				// under the same function name in one model request.
				filtered := nativeTools[:0]
				for _, tool := range nativeTools {
					legacyReady := state.step == 0 && controlObserved
					if task.Config.ComputerScope == "desktop" {
						legacyReady = legacyReady && nativeDocumentControlFallback(task)
					}
					if legacyReady || tool.Function.Name == "computer_observe" {
						filtered = append(filtered, tool)
					}
				}
				nativeTools = append(filtered, stagedDocumentTools(task)...)
			}
			for _, tool := range nativeTools {
				if task.Config.ComputerScope != "desktop" || desktopPlanningTool(task, tool.Function.Name) {
					tools = append(tools, tool)
				}
			}
			if task.Config.ComputerScope == "desktop" {
				state, _ := documentState(task)
				_, _, selected := desktopPlanningState(task)
				for _, tool := range desktopComputerTools() {
					// Inspect the newly selected editor before returning to its app
					// catalog. This read establishes the available document/control
					// operations; it neither executes input nor prevents later app
					// switching once that observation (or unsupported result) exists.
					if documentsAvailable && selected && state.step == 0 && !nativeDocumentControlFallback(task) && (tool.Function.Name == "computer_applications" || tool.Function.Name == applicationSearchTool) {
						continue
					}
					if (!documentsAvailable || state.step == 0 || tool.Function.Name == "computer_windows") && desktopPlanningTool(task, tool.Function.Name) {
						tools = append(tools, tool)
					}
				}
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
	if task.Config.Profile != nil {
		permitted := tools[:0]
		for _, tool := range tools {
			d, ok := b.Capability(tool.Function.Name)
			if ok && b.server.profileActorCapabilityAllowed(task.Actor, d) {
				permitted = append(permitted, tool)
			}
		}
		tools = permitted
	}
	return tools
}
