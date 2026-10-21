package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Help constrained decoders copy opaque references exactly. This is only a
// model-view restriction: dispatch still checks fresh observations and consent.
// Desktop tasks already use task-local short aliases; do not mix their schema
// with canonical selected-window IDs.
func selectedWindowToolReferences(task *agents.Task, tool api.Tool) api.Tool {
	if !task.Config.TaskFirst || task.Config.ComputerScope == "desktop" || !nativeComputerDriver(task.Config.ComputerDriver) {
		return tool
	}
	var observation string
	for i := len(task.Steps) - 1; i >= task.Config.ComputerStepOffset && i >= 0; i-- {
		step := task.Steps[i]
		if step.Type == "action" && step.ToolName == "computer_observe" {
			var data computer.NativeView
			if decodeNativeModelResult(step.ToolResult, &data) {
				observation = data.Observation.ID
			}
			break
		}
	}
	view, err := savedDiscoveryView(task, observation)
	if err != nil {
		return tool
	}
	params := tool.Function.Parameters
	properties, ok := params["properties"].(map[string]interface{})
	if !ok || properties["observation_id"] == nil {
		return tool
	}
	copyParams := make(map[string]interface{}, len(params))
	for k, v := range params {
		copyParams[k] = v
	}
	copyProperties := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		copyProperties[k] = v
	}
	copyProperties["observation_id"] = map[string]any{"type": "string", "enum": []string{observation}}
	var ids []string
	for _, e := range view.Elements {
		eligible := false
		switch tool.Function.Name {
		case "computer_replace_text":
			eligible = e.Writable
		case "computer_activate":
			eligible = e.Invokable
		case "computer_set_checked", "computer_verify_checked":
			eligible = e.Checkable
		case "computer_verify", "computer_inspect_control", "computer_shortcut":
			eligible = true
		}
		if eligible {
			ids = append(ids, e.ID)
		}
	}
	// Keep schema growth bounded. Discovery/inspection and authoritative target
	// validation remain available for large surfaces; never invent a selection.
	if properties["element"] != nil && len(ids) > 0 && len(ids) <= 32 {
		copyProperties["element"] = map[string]any{"type": "string", "enum": ids}
	}
	copyParams["properties"] = copyProperties
	tool.Function.Parameters = copyParams
	return tool
}
