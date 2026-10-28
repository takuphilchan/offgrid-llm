package agents

import (
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Narrow a rejected write's next proposal to an independently observed preimage.
// Nothing is dispatched or rewritten here. Inference still chooses whether to
// call; the ordinary host compare-and-write, authority and receipt checks remain.
func projectWorkspaceWriteRepair(task *Task, tools []api.Tool) []api.Tool {
	cp := task.Checkpoint
	if cp == nil || !cp.PreEffectRepair || task.Config.Execution == nil || (cp.PreEffectRepairTool != "workspace_write" && cp.PreEffectRepairTool != execution.InputWriteTool) {
		return tools
	}
	target, revision, rejected := "", "", -1
	var selection execution.InputWriteArguments
	for i := len(task.Steps) - 1; i >= 0; i-- {
		s := task.Steps[i]
		if s.Type != "rejected" || s.ToolName != cp.PreEffectRepairTool {
			continue
		}
		var args struct{ Path string }
		var failure struct{ Error, Effects string }
		if json.Unmarshal([]byte(s.ToolArgs), &args) != nil || json.Unmarshal([]byte(s.ToolResult), &failure) != nil || failure.Error != "execution_write_precondition" || failure.Effects != "none" || !repairRelativePath(args.Path) {
			return tools
		}
		target, rejected = path.Clean(args.Path), i
		if cp.PreEffectRepairTool == execution.InputWriteTool {
			selection, _ = execution.ParseInputWrite(json.RawMessage(s.ToolArgs))
		}
		break
	}
	if rejected < 0 {
		return tools
	}
	for i := rejected + 1; i < len(task.Steps); i++ {
		s := task.Steps[i]
		if execution.MutatingOperation(execution.Operation(s.ToolName)) {
			return tools
		}
		if s.Type != "action" || s.ToolName != "workspace_read" {
			continue
		}
		var args struct{ Path string }
		if json.Unmarshal([]byte(s.ToolArgs), &args) != nil || !repairRelativePath(args.Path) || path.Clean(args.Path) != target {
			continue
		}
		ref, ok := executionReferenceAtStep(task, i)
		if !ok || ref != *task.Config.Execution {
			return tools
		}
		var view struct {
			Host, Session, Operation string
			Receipt                  execution.Receipt
		}
		var page execution.FilePage
		if json.Unmarshal([]byte(s.ToolResult), &view) != nil || view.Host != ref.Host || view.Session != ref.Session || view.Operation != "file_read" || view.Receipt.State != "complete" || json.Unmarshal(view.Receipt.Result, &page) != nil || page.Host != ref.Host {
			return tools
		}
		decoded, err := hex.DecodeString(page.Revision)
		if err != nil || len(decoded) != 32 || page.Revision != strings.ToLower(page.Revision) {
			return tools
		}
		revision = page.Revision
	}
	if revision == "" {
		return tools
	}
	projected := append([]api.Tool(nil), tools...)
	for i, tool := range projected {
		if tool.Function.Name != cp.PreEffectRepairTool {
			continue
		}
		// Copy the schema; never mutate the shared catalog or saved arguments.
		raw, _ := json.Marshal(tool)
		var narrowed api.Tool
		if json.Unmarshal(raw, &narrowed) != nil {
			return tools
		}
		fields, ok := narrowed.Function.Parameters["properties"].(map[string]any)
		if !ok {
			return tools
		}
		fields["path"] = map[string]any{"type": "string", "const": target}
		fields["revision"] = map[string]any{"type": "string", "const": revision}
		if cp.PreEffectRepairTool == execution.InputWriteTool && selection.Input != "" {
			for name, value := range map[string]string{"input": selection.Input, "after": selection.After, "before": selection.Before} {
				fields[name] = map[string]any{"type": "string", "const": value}
			}
		}
		required, _ := narrowed.Function.Parameters["required"].([]any)
		for _, name := range []string{"path", "revision"} {
			found := false
			for _, item := range required {
				found = found || item == name
			}
			if !found {
				required = append(required, name)
			}
		}
		narrowed.Function.Parameters["required"] = required
		narrowed.Function.Description += " Repair this target using its just-observed preimage. The host still rejects concurrent changes; do not replay completed or uncertain writes."
		projected[i] = narrowed
	}
	if cp.PreEffectRepairTool == execution.InputWriteTool {
		// The current preimage has been read and the rejected action had no
		// effects. Present only that exact-input repair: another read cannot
		// advance it, and a plain write can lose the selected bytes.
		visible := projected[:0]
		for _, tool := range projected {
			if tool.Function.Name == execution.InputWriteTool {
				visible = append(visible, tool)
			}
		}
		return visible
	}
	return projected
}

func repairRelativePath(value string) bool {
	clean := path.Clean(value)
	return value != "" && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !path.IsAbs(clean) && !strings.ContainsAny(value, "\\:\x00")
}
