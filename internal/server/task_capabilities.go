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
	// A selected-window mutation cannot be planned without its first real
	// observation. Match the existing dispatch guard in the offered schema, so
	// small models are not encouraged to invent element/observation references.
	initialNativeObservation := task.Config.TaskFirst && task.Config.ComputerSession != "" && nativeComputerDriver(task.Config.ComputerDriver) && task.Config.ComputerScope != "desktop"
	if initialNativeObservation {
		for i, step := range task.Steps {
			if i < task.Config.ComputerStepOffset || step.Type != "action" {
				continue
			}
			if invalidatesNativeCheck(step.ToolName, "control_text_equals") {
				initialNativeObservation = true
			}
			if i >= task.Config.ComputerStepOffset && step.Type == "action" && (step.ToolName == "computer_observe" || step.ToolName == "computer_documents") {
				initialNativeObservation = false
			}
		}
	}
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
			// Host negotiation alone must not steer a model into the document
			// protocol for browser/search/control work. Route it only when the
			// requested outcome actually mentions document/tab editing or reading.
			documentsAvailable := b.documentToolsAvailable(task) && nativeDocumentTaskIntent(task)
			recoveryPending := nativeDocumentObservationRecoveryPending(task)
			if documentsAvailable {
				state, _ := documentState(task)
				controlObserved := task.Config.ComputerScope != "desktop"
				for i, step := range task.Steps {
					if i >= task.Config.ComputerStepOffset && step.Type == "action" && step.ToolName == "computer_observe" {
						controlObserved = true
					}
					if i >= task.Config.ComputerStepOffset && (step.Type == "action" || step.Type == "tool_error") && (step.ToolName == "computer_open_application" || step.ToolName == "computer_focus_window" || step.ToolName == "computer_documents") {
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
				nativeTools = filtered
				if !recoveryPending {
					nativeTools = append(nativeTools, stagedDocumentTools(task)...)
				}
			}
			for _, tool := range nativeTools {
				if recoveryPending && !nativeDocumentRecoveryToolAllowed(tool.Function.Name) {
					continue
				}
				if initialNativeObservation && tool.Function.Name != "computer_observe" && tool.Function.Name != "computer_documents" {
					continue
				}
				tool = selectedWindowToolReferences(task, tool)
				if task.Config.ComputerScope != "desktop" || desktopPlanningTool(task, tool.Function.Name) {
					tools = append(tools, tool)
				}
			}
			if task.Config.ComputerScope == "desktop" {
				state, _ := documentState(task)
				_, _, selected := desktopPlanningState(task)
				for _, tool := range desktopComputerTools() {
					if recoveryPending && !nativeDocumentRecoveryToolAllowed(tool.Function.Name) {
						continue
					}
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
		if task.Config.ComputerScope != "desktop" && !initialNativeObservation {
			tools = append(tools, computerAccessTool())
		}
		if b.server.artifactStore != nil && !agents.ComputerAccessNeedsObservation(task, b) {
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
