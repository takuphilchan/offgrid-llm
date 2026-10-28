package server

import (
	"encoding/json"
	"path"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Narrow model decisions to continuations actually returned by this job's host.
// This is a prompt/decision constraint, not authority or freshness proof: the
// independent host still checks grants, paths and revisions at dispatch.
func workspacePageTool(tool api.Tool, task *agents.Task) api.Tool {
	operation := executionOperation(tool.Function.Name)
	if operation != "file_list" && operation != "file_read" || task.Config.Execution == nil {
		return tool
	}
	parameters := make(map[string]any, len(tool.Function.Parameters))
	for key, value := range tool.Function.Parameters {
		parameters[key] = value
	}
	branches, ok := parameters["oneOf"].([]any)
	if !ok || len(branches) == 0 {
		return tool
	}
	choices := []any{branches[0]}
	seen := map[string]bool{}
	for i := len(task.Steps) - 1; i >= max(0, len(task.Steps)-128); i-- {
		step := task.Steps[i]
		if step.Type != "action" {
			continue
		}
		if step.ToolName == "workspace_change_directory" || executionOperation(step.ToolName) == "file_write" || step.ToolName == "workspace_process_start" {
			break
		}
		if step.ToolName != tool.Function.Name || len(step.ToolResult) > 1<<20 || jsonwire.Validate([]byte(step.ToolResult)) != nil {
			continue
		}
		var args struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
		}
		if json.Unmarshal([]byte(step.ToolArgs), &args) != nil {
			continue
		}
		if args.Path == "" {
			args.Path = "."
		}
		key := path.Clean(args.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		var result struct {
			Host      string `json:"host"`
			Session   string `json:"session"`
			Operation string `json:"operation"`
			Receipt   struct {
				State  string `json:"state"`
				Result struct {
					Revision   string `json:"revision"`
					NextOffset *int   `json:"next_offset"`
					Complete   *bool  `json:"complete"`
				} `json:"result"`
			} `json:"receipt"`
		}
		if json.Unmarshal([]byte(step.ToolResult), &result) != nil || result.Operation != operation || result.Host != task.Config.Execution.Host || result.Session != task.Config.Execution.Session || result.Receipt.State != "complete" {
			continue
		}
		page := result.Receipt.Result
		if page.Complete == nil || *page.Complete || page.NextOffset == nil || *page.NextOffset <= args.Offset || len(page.Revision) != 64 {
			continue
		}
		// Copy the complete fresh object, then replace only the three observed
		// values. Bare refinement branches lose path/limit in grammar decoding.
		fresh := branches[0].(map[string]any)
		fields := fresh["properties"].(map[string]any)
		properties := make(map[string]any, len(fields))
		for name, field := range fields {
			copy := map[string]any{}
			for key, value := range field.(map[string]any) {
				copy[key] = value
			}
			properties[name] = copy
		}
		for name, value := range map[string]any{"path": args.Path, "offset": *page.NextOffset, "revision": page.Revision} {
			properties[name].(map[string]any)["const"] = value
		}
		choices = append(choices, map[string]any{"type": "object", "additionalProperties": false, "required": []string{"path", "offset", "revision"}, "properties": properties})
		if len(choices) == 5 {
			break
		} // Four recent targets; older targets can be read afresh.
	}
	parameters["oneOf"] = choices
	tool.Function.Parameters = parameters
	return tool
}
