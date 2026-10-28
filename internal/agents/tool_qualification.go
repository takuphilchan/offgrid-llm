package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Explicit inventory, never prefixes or provider-supplied read-only annotations.
func scopedToolName(name string) bool {
	switch name {
	case "task_plan", "task_history", "task_find_tools", "skill_load",
		execution.AccessRequestTool, "workspace_info", "workspace_list", "workspace_read",
		"workspace_write", execution.InputWriteTool, "workspace_process_start",
		"workspace_process_wait", "workspace_process_output":
		return true
	}
	return false
}

// The evidence binds the complete reviewed schemas and capability identities.
// A schema/provider change cannot inherit qualification just by keeping a name.
var nativeWorkspaceCatalogOnce sync.Once
var nativeWorkspaceCatalog string

func NativeWorkspaceCapabilitySHA256() string {
	nativeWorkspaceCatalogOnce.Do(func() { nativeWorkspaceCatalog = measureNativeWorkspaceCatalog() })
	return nativeWorkspaceCatalog
}

func measureNativeWorkspaceCatalog() string {
	type entry struct {
		Tool       api.Tool                `json:"tool"`
		Descriptor capabilities.Descriptor `json:"descriptor"`
	}
	var entries []entry
	for _, tool := range execution.ModelTools() {
		if scopedToolName(tool.Function.Name) {
			d, _ := execution.Descriptor(tool.Function.Name)
			entries = append(entries, entry{tool, d})
		}
	}
	for _, tool := range agentcontext.Tools() {
		if scopedToolName(tool.Function.Name) {
			d, _ := agentcontext.Descriptor(tool.Function.Name)
			entries = append(entries, entry{tool, d})
		}
	}
	// Nil pin avoids recursion and projects only stable internal tools.
	for _, tool := range runtimeTools(&Task{Config: AgentConfig{TaskFirst: true}}) {
		if scopedToolName(tool.Function.Name) {
			entries = append(entries, entry{Tool: tool})
		}
	}
	entries = append(entries, entry{execution.AccessRequestSchema(), execution.AccessRequestDescriptor()})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Tool.Function.Name < entries[j].Tool.Function.Name })
	encoded, _ := json.Marshal(entries)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func ValidateAgentToolScope(pin *inference.AgentToolPin) error {
	if pin == nil {
		return nil
	}
	if pin.Adapter != inference.NativeAgentToolAdapter {
		return errors.New("unknown agent tool adapter")
	}
	if pin.Scope == "" {
		if pin.CapabilitySHA256 != "" {
			return errors.New("unexpected capability scope inventory")
		}
		return nil // Historical full-suite pins keep their original boundary.
	}
	if _, knownScope := models.NativeWorkspaceCasesForScope(pin.Scope); !knownScope || pin.CapabilitySHA256 != NativeWorkspaceCapabilitySHA256() {
		return errors.New("native tool qualification scope is unknown or its capability inventory changed")
	}
	return nil
}

func qualifiedTool(task *Task, name string, actual capabilities.Descriptor, known bool) bool {
	pin := task.Config.ToolAdapter
	if ValidateAgentToolScope(pin) != nil {
		return false
	}
	if pin == nil || pin.Scope == "" {
		return true
	}
	if !scopedToolName(name) {
		return false
	}
	if isRuntimeTool(name) {
		return true
	} // These names dispatch only inside Runner.
	if !known {
		return false
	}
	if name == execution.AccessRequestTool {
		return actual == execution.AccessRequestDescriptor()
	}
	if name == "skill_load" {
		want, ok := agentcontext.Descriptor(name)
		return ok && actual == want
	}
	want, ok := execution.Descriptor(name)
	return ok && actual == want
}

// Projectors may alter schemas after discovery. Reapply the same boundary before
// counting/generation so an additional presentation path cannot widen the menu.
func (r *Runner) qualifiedTools(task *Task, tools []api.Tool) []api.Tool {
	filtered := make([]api.Tool, 0, len(tools))
	for _, tool := range tools {
		d, ok := r.tools.Capability(tool.Function.Name)
		if qualifiedTool(task, tool.Function.Name, d, ok) {
			filtered = append(filtered, tool)
		}
	}
	// Host/provider catalogs may originate in maps. Native adapters must see a
	// stable final order after ranking/projection, both for repeatable prompts
	// and prefix-cache reuse. This changes neither membership nor schemas; old
	// constrained jobs retain their presentation contract.
	if pin := task.Config.ToolAdapter; pin != nil && pin.Adapter == inference.NativeAgentToolAdapter {
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Function.Name < filtered[j].Function.Name })
	}
	return filtered
}
