package execution

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Tools is the authoritative reviewed model-facing workspace catalog. It is
// separate from service files and from arbitrary configured shell/script tools.
func Tools() []api.Tool {
	str := map[string]any{"type": "string"}
	path := map[string]any{"type": "string", "pattern": `^([^/\\:\x00][^\\:\x00]*)?$`, "description": "Relative forward-slash path: ., reports/file.txt. Never copy absolute metadata paths."}
	revision := map[string]any{"type": "string", "pattern": "^([a-f0-9]{64})?$", "description": "Omit/empty for fresh reads or creation. Otherwise use this target's exact observed SHA-256, never its parent's."}
	integer := map[string]any{"type": "integer", "minimum": 0}
	tool := func(name, description string, fields map[string]any, required ...string) api.Tool {
		parameters := map[string]any{"type": "object", "additionalProperties": false, "properties": fields}
		if len(required) > 0 {
			parameters["required"] = required
		}
		if name == "workspace_list" || name == "workspace_read" {
			// The pinned grammar compiler visits union branches before siblings.
			// Each branch must therefore carry the whole object contract, including
			// path and required fields, rather than just offset/revision refinements.
			fresh := pageObject(fields, required)
			continuation := pageObject(fields, append(append([]string{}, required...), "offset", "revision"))
			fresh["properties"].(map[string]any)["offset"].(map[string]any)["const"] = 0
			fresh["properties"].(map[string]any)["revision"].(map[string]any)["const"] = ""
			continuation["properties"].(map[string]any)["offset"].(map[string]any)["minimum"] = 1
			continuation["properties"].(map[string]any)["revision"].(map[string]any)["minLength"] = 64
			parameters = map[string]any{"oneOf": []any{fresh, continuation}}
		}
		return api.Tool{Type: "function", Function: api.FunctionDef{Name: name, Description: description, Parameters: parameters}}
	}
	page := func(maxLimit, maxOffset int) map[string]any {
		return map[string]any{"path": path, "offset": map[string]any{"type": "integer", "minimum": 0, "maximum": maxOffset, "description": "First page: omit/0. Continue with this target's next_offset and revision."}, "limit": map[string]any{"type": "integer", "minimum": 0, "maximum": maxLimit, "description": "Page size; omit/0 uses default."}, "revision": revision}
	}
	return []api.Tool{
		tool("workspace_info", "Inspect the explicitly approved execution host, root and current directory. This is not the service filesystem.", map[string]any{}),
		tool("workspace_list", "List approved workspace entries, not file contents. First call: {\"path\":\".\"}. New targets omit revision; continuation uses their own next_offset/revision.", page(MaxEntriesPerPage, MaxDirectoryEntries)),
		tool("workspace_read", "Read exact UTF-8 text without line prefixes; data is original base64 bytes, including binary/split characters. Follow next_offset/revision to complete a file. Never treat partial pages as complete or repeat mutations.", page(MaxPageBytes, MaxFileBytes), "path"),
		tool("workspace_search", "Bounded literal case-sensitive workspace search. Previews may be incomplete; use workspace_read for exact copying.", map[string]any{"path": path, "query": str, "limit": integer}, "query"),
		tool("workspace_write", "Write exact UTF-8: create exclusively or replace using workspace_read's preimage revision. Returns independent read-back. Never retry an uncertain write.", map[string]any{"path": path, "content": map[string]any{"type": "string", "maxLength": 131072}, "revision": revision}, "path", "content"),
		tool(InputWriteTool, "Write exact bytes between unique after/before markers (excluded) in an offered input. Independent read-back; never retry uncertain effects.", map[string]any{"path": map[string]any{"type": "string", "pattern": path["pattern"], "description": "Relative forward-slash path within the approved folder."}, "input": map[string]any{"type": "string", "maxLength": 160}, "after": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "description": "Opening marker. Copy starts after it."}, "before": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "description": "Closing marker after the opening. Copy ends before it."}, "revision": revision}, "path", "input", "after", "before"),
		tool("workspace_change_directory", "Change only this execution session's directory within its approved root. Does not change the service or other sessions.", map[string]any{"path": path}, "path"),
		tool("workspace_process_start", "Run a command with separate arguments in the approved session cwd. Trusted mode uses your OS account, not a sandbox. Reviewed project tests/builds use session authority; other commands need exact approval. Inspect exit/output before claiming success. Never retry an uncertain start.", map[string]any{"program": map[string]any{"type": "string", "description": "For ordinary project tests/builds use the host PATH command name, e.g. go, npm or git. Absolute/custom executable paths require exact approval; do not invent one."}, "arguments": map[string]any{"type": "array", "items": str, "maxItems": 128}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": 3600, "description": "Omit/0 uses the 180-second default; increase only when the task requires it."}}, "program", "arguments"),
		tool("workspace_process_output", "Read owned process output without re-running. Follow next_offset; output_truncated means incomplete capture. Running/failed is not success.", map[string]any{"handle": str, "offset": integer, "limit": integer}, "handle"),
		tool("workspace_process_wait", "Wait briefly for an owned process without restarting it. Check actual exit_code and output before claiming success.", map[string]any{"handle": str, "wait_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 1000}}, "handle"),
		tool("workspace_process_stop", "Request stopping only this job's owned process tree. State remains running until termination is observed. Stopping cannot undo file or network effects.", map[string]any{"handle": str, "wait_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 1000}}, "handle"),
	}
}

// ModelTools adds argument help without changing registered capability identity
// or the argument grammar of existing saved calls. Authorization uses Tools.
func ModelTools() []api.Tool {
	tools := Tools()
	for i := range tools {
		tool := &tools[i]
		if tool.Function.Name == "workspace_process_wait" {
			tool.Function.Description += " If the recorded process is running, prefer wait_ms=1000 to repeated immediate status/output polls. Waiting observes the same process; it never starts it again."
		}
		if tool.Function.Name == "workspace_write" || tool.Function.Name == InputWriteTool {
			tool.Function.Description += " To replace an existing file, first workspace_read that target in this turn and copy its exact revision into the write. Empty revision means create-only and cannot overwrite. Earlier conversation answers are not a current preimage."
			fields := tool.Function.Parameters["properties"].(map[string]any)
			fields["revision"] = map[string]any{"type": "string", "pattern": "^([a-f0-9]{64})?$", "description": "For an existing target, REQUIRED: exact revision from this turn's workspace_read of that file. Empty/omitted is only for a new file."}
		}
		if tool.Function.Name == "workspace_process_start" {
			fields := tool.Function.Parameters["properties"].(map[string]any)
			fields["arguments"].(map[string]any)["description"] = "Arguments AFTER the executable, not the full command. For program go and command go test ./..., use [\"test\",\"./...\"]. Never repeat the program in this array or combine arguments into a shell string."
		}
	}
	return tools
}

func pageObject(fields map[string]any, required []string) map[string]any {
	properties := make(map[string]any, len(fields))
	for name, field := range fields {
		copy := map[string]any{}
		for key, value := range field.(map[string]any) {
			copy[key] = value
		}
		properties[name] = copy
	}
	result := map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
	if len(required) > 0 {
		result["required"] = append([]string{}, required...)
	}
	return result
}

func Operation(name string) string {
	switch name {
	case "workspace_info":
		return "workspace_info"
	case "workspace_list":
		return "file_list"
	case "workspace_read":
		return "file_read"
	case "workspace_search":
		return "file_search"
	case "workspace_write", InputWriteTool:
		return "file_write"
	case "workspace_change_directory":
		return "directory_change"
	case "workspace_process_start":
		return "process_start"
	case "workspace_process_output":
		return "process_inspect"
	case "workspace_process_wait":
		return "process_wait"
	case "workspace_process_stop":
		return "process_stop"
	}
	return ""
}

func Descriptor(name string) (capabilities.Descriptor, bool) {
	for _, tool := range Tools() {
		if tool.Function.Name == name {
			encoded, _ := json.Marshal(tool)
			sum := sha256.Sum256(encoded)
			kind := capabilities.Read
			if name == "workspace_write" || name == InputWriteTool || name == "workspace_change_directory" {
				kind = capabilities.Write
			}
			if name == "workspace_process_start" || name == "workspace_process_stop" {
				kind = capabilities.Execute
			}
			return capabilities.Descriptor{Name: name, Namespace: "execution", Source: "local-executor", Location: "host", Revision: fmt.Sprintf("execution-v1-%x", sum), Kind: kind, Risk: capabilities.RiskMedium, Description: tool.Function.Description}, true
		}
	}
	return capabilities.Descriptor{}, false
}

func Permitted(name string, p Permissions) bool {
	if name == "workspace_process_start" || name == "workspace_process_stop" || name == "workspace_process_wait" || name == "workspace_process_output" {
		return p.Processes
	}
	if name == "workspace_write" || name == InputWriteTool {
		return p.Write
	}
	return Operation(name) != "" && p.Read
}

// FileReadTool identifies only reviewed non-mutating workspace observations.
// Process inspection and unknown provider operations do not inherit this policy.
func FileReadTool(name string) bool {
	return name == "workspace_info" || name == "workspace_list" || name == "workspace_read" || name == "workspace_search"
}
