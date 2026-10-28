package agents

import (
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"sort"
)

// Presentation context, never dispatch authority. Keep user messages and tool
// evidence intact. Names come from the effective catalog, not model prose.
func responseContext(task *Task, tools []api.Tool) string {
	if task.Config.ExecutionRequestVersion != execution.AccessRequestVersion {
		return ""
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	sort.Strings(names)
	if len(names) > 32 {
		names = names[:32]
	}
	data, _ := json.Marshal(names)
	text := "\nRespond to the user's requested outcome, not to the tool catalog. You can converse, explain, reason, draft and tell stories directly. Tools are optional means to obtain requested evidence or perform requested actions. A question about your capabilities asks for an explanation of available abilities, not a demonstration or a workspace inspection. Currently offered tool names (bounded selection, not all possible integrations): " + string(data) + ". Describe only these and the effective attachments; distinguish requestable access from granted access. Tool errors and repair guidance are internal context: decide whether the failed operation was needed for the user's request. If it was unnecessary, answer the original request directly. Do not return repair instructions as the user's answer or invent a dependency on a missing file."
	if task.Config.ExecutionWorkspace != nil {
		for _, name := range names {
			if name == execution.AccessRequestTool {
				text += "\nThe attached folder's granted_permissions describe the CURRENT access, not all requestable capabilities. If the requested work needs a missing permission, call request_workspace_access alone: edit enables file creation/changes; trusted enables commands. Wait for the client's decision, then perform the original work. A read-only folder does not allow writing, and reading a nonexistent destination does not create it. Do not replace a requested write or command with a read or an explanation of setup. If access is refused, explain the limitation without claiming completion."
				break
			}
		}
	}
	// Reiterate the actual request after observations AND repair feedback, never
	// a model-written summary. Long requests remain in full in original history.
	if len(task.Checkpoint.Messages) > 0 && (task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].Role == "tool" || task.Checkpoint.DecisionRepairs > 0) {
		goal, _ := json.Marshal(struct {
			Request   string `json:"request"`
			Truncated bool   `json:"truncated"`
		}{resourceLabel(task.Prompt, 1200), len([]rune(task.Prompt)) > 1200})
		text += "\nCurrent turn's original request (quoted reminder, not a replacement for the original or subsequent user corrections): " + string(goal)
		if task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1].Name == execution.AccessRequestTool {
			text += "\nWorkspace access has just been attached; the access result is NOT a directory listing, file read, edit or command result. It only establishes availability. If the original request needs workspace operations, continue that work using the newly offered workspace tools. If access was unnecessary for a conversational request, answer directly without inventing file work. A folder label or '.' is not a list of its entries. Do not report the permission/setup step as the completed outcome. If the requested operation cannot be performed, explain that limitation instead."
		}
	}
	return text
}
