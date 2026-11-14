package agents

import (
	"sort"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func profileDelegationTool(task *Task) api.Tool {
	tool := delegationTool()
	if task.Config.Profile == nil && task.WorkID == "" {
		return tool
	}
	ids := make([]string, 0, len(task.Config.DelegateProfiles))
	for id := range task.Config.DelegateProfiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	items := tool.Function.Parameters["properties"].(map[string]interface{})["tasks"].(map[string]interface{})["items"].(map[string]interface{})
	if task.WorkID == "" {
		items["required"] = []string{"key", "goal", "depends_on", "tools", "profile_id"}
	} else {
		items["required"] = []string{"key", "goal", "expected_result", "reason", "depends_on", "tools"}
		properties := items["properties"].(map[string]interface{})
		properties["expected_result"] = map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 4096, "description": "Deliverable and evidence needed by the parent; not independent verification."}
		properties["reason"] = map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 2048, "description": "Why separate worker context or capability justifies its resource cost."}
	}
	if len(ids) > 0 {
		items["properties"].(map[string]interface{})["profile_id"] = map[string]interface{}{"type": "string", "enum": ids}
	}
	// The model must not invent a tool name from the global catalog. The
	// effective child capability intersection is the only source for this
	// field; Knowledge-only specialists therefore receive an explicit empty
	// tools array and can work from the retrieved excerpts in their context.
	allowed := map[string]bool{}
	executionEnabled := task.WorkID != "" && task.Config.Execution != nil && task.Config.ExecutionWorkspace != nil && task.Config.ExecutionWorkspace.Backend == "local" && task.Config.ExecutionWorkspace.PreparedWorkers
	if !executionEnabled {
		items["properties"].(map[string]interface{})["mode"] = map[string]interface{}{"type": "string", "enum": []string{"research"}}
	} else {
		items["properties"].(map[string]interface{})["mode"] = map[string]interface{}{"type": "string", "enum": []string{"research", "edit", "test"}}
	}
	if task.WorkID != "" {
		for name, descriptor := range task.Config.AllowedCapabilities {
			if (delegatedCapability(name, descriptor) || executionEnabled && executionAssignmentTool("edit", name, descriptor)) && ProfileAllowsCapability(task.Config, name, descriptor) {
				allowed[name] = true
			}
		}
	}
	for _, child := range task.Config.DelegateProfiles {
		for name, descriptor := range child.AllowedCapabilities {
			if delegatedCapability(name, descriptor) && ProfileAllowsCapability(task.Config, name, descriptor) {
				allowed[name] = true
			}
		}
	}
	var names []string
	for name := range allowed {
		names = append(names, name)
	}
	sort.Strings(names)
	toolsSchema := items["properties"].(map[string]interface{})["tools"].(map[string]interface{})
	if len(names) > 0 {
		toolsSchema["items"].(map[string]interface{})["enum"] = names
	}
	if len(names) == 0 {
		toolsSchema["maxItems"] = 0
		tool.Function.Description += " This parent has no approved child tools; set tools to [] and use the attached Knowledge excerpts."
	} else if task.WorkID == "" {
		tool.Function.Description += " Select a pinned child profile_id. Child calls share the parent's total iteration budget; choose only the displayed approved tools."
	}
	if task.WorkID != "" {
		tool.Function.Description += " Reusable specialist profiles are optional. Temporary workers share the parent's total budget and approved resource ceiling."
	}
	if executionEnabled {
		tool.Function.Description = "Delegate bounded research, edit or test assignments when useful. Research uses read-only tools. Edit/test workers receive independent working copies through the authorized host; use workspace tools for their files. Test workers cannot use workspace_write, but trusted test processes can produce files. Changes are proposals until separately validated and integrated. Workers cannot acquire grants, control the desktop, delegate, or bypass the parent capability ceiling. Prefer continuing a relevant worker. Report unavailable preparation explicitly; never fall back to the shared original folder."
	}
	if task.WorkID != "" {
		policy := resolvedWorkerPolicy(task)
		tool.Function.Parameters["properties"].(map[string]interface{})["tasks"].(map[string]interface{})["maxItems"] = min(4, policy.MaxSessions)
		tool.Function.Description += " depends_on may name a unique, current completed assignment key from this attempt; its exact result is reused without rerunning that worker. Use this to test a retained editor's follow-up."
	}
	return tool
}
