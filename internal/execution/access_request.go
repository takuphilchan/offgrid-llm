package execution

import (
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const AccessRequestVersion = 1
const AccessRequestTool = "request_workspace_access"

func AccessRequestSchema() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: AccessRequestTool, Description: AccessRequestDescriptor().Description, Parameters: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"access"}, "properties": map[string]any{"access": map[string]any{"type": "string", "enum": []string{"read", "edit", "trusted"}}}}}}
}

// This descriptor requests a trusted-client decision; it cannot execute files
// or nominate a host path. It is deliberately separate from a worker grant.
func AccessRequestDescriptor() capabilities.Descriptor {
	return capabilities.Descriptor{Name: AccessRequestTool, Namespace: "execution-access", Source: "workspace", Kind: capabilities.Read, Risk: capabilities.RiskLow, Revision: "execution-access-v1", Description: "Request access to the user's local workspace only when their task needs local files or commands. Choose read, edit, or trusted. The trusted client chooses the folder and obtains permission; this request itself grants nothing. Not needed for conversation, explanation, creative writing, or describing capabilities. Call alone."}
}

func AccessPermissions(mode string) (Permissions, bool) {
	switch mode {
	case "read":
		return Permissions{Read: true}, true
	case "edit":
		return Permissions{Read: true, Write: true}, true
	case "trusted":
		return Permissions{Read: true, Write: true, Processes: true}, true
	}
	return Permissions{}, false
}

func PermissionsWithin(want, ceiling Permissions) bool {
	return (!want.Read || ceiling.Read) && (!want.Write || ceiling.Write) && (!want.Processes || ceiling.Processes)
}
