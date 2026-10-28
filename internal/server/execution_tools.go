package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Names are deliberately distinct from the legacy service-filesystem tools.
// Models cannot choose an execution host by passing another path or host ID.
func executionTools() []api.Tool {
	return execution.ModelTools()
}

func executionOperation(name string) string {
	return execution.Operation(name)
}

func executionDescriptor(name string) (capabilities.Descriptor, bool) {
	return execution.Descriptor(name)
}

func executionPermitted(name string, p execution.Permissions) bool {
	return execution.Permitted(name, p)
}

func authorizeExecutionInvocation(name string, args json.RawMessage, e agents.ToolExecution) error {
	if name != "workspace_process_start" {
		return nil
	}
	spec, err := execution.ParseProcessSpec(args)
	if err != nil {
		return &agents.ToolAuthorizationError{Code: "execution_arguments_invalid", Message: "Provide one executable with separate arguments and a bounded timeout. No command was dispatched.", Cause: err}
	}
	if !e.Approved && execution.RepeatsRoutineProgram(spec) {
		return &agents.ToolAuthorizationError{Code: "execution_program_repeated", Message: "The arguments repeat the executable before a reviewed project command. Correct the separate program and arguments fields. No command was dispatched."}
	}
	if !e.Approved && !execution.RoutineProcess(spec) {
		return fmt.Errorf("%w: this command is outside the reviewed routine-project grammar; review its exact executable and arguments under the selected host account", capabilities.ErrApprovalRequired)
	}
	return nil
}

func (b *browserRunTools) executionAccess(ctx context.Context, task *agents.Task, name string, e agents.ToolExecution) error {
	if name == execution.InputWriteTool && task.Config.InputWriteVersion != execution.InputWriteVersion {
		return capabilities.ErrDenied
	}
	d, ok := executionDescriptor(name)
	if !ok || e.ExpectedCapability == nil || *e.ExpectedCapability != d || task.Actor != e.Actor || task.ID != e.RunID || task.Status != agents.TaskRunning || task.ParentID != "" || !task.Config.TaskFirst || task.Config.Execution == nil || b.server.executionHosts == nil {
		return capabilities.ErrDenied
	}
	if !b.server.profileActorCapabilityAllowed(task.Actor, d) || !agents.ProfileAllowsCapability(task.Config, name, d) {
		return capabilities.ErrDenied
	}
	if task.Config.ExecutionPolicy != nil && !executionPermitted(name, *task.Config.ExecutionPolicy) {
		return capabilities.ErrDenied
	}
	resource, err := b.server.executionHosts.Resolve(ctx, task.Actor, b.server.workspaceID, *task.Config.Execution)
	if err != nil || !executionScopeMatches(task, resource) || !executionPermitted(name, resource.Permissions) || !executionPermitted(name, task.Config.ExecutionWorkspace.Permissions) {
		return &agents.ToolAuthorizationError{Code: "execution_unavailable", Message: "The selected execution host or folder grant is unavailable. No service-directory fallback or action was dispatched.", Cause: err}
	}
	return nil
}

func (b *browserRunTools) executeWorkspace(ctx context.Context, task *agents.Task, name string, args json.RawMessage, e agents.ToolExecution) (string, error) {
	if name == "workspace_write" && task.Checkpoint != nil && task.Checkpoint.PreEffectRepair && task.Checkpoint.PreEffectRepairTool == execution.InputWriteTool {
		return "", &agents.ToolFailure{Code: "execution_input_repair_pending", Message: "The pending exact-input write must be repaired with workspace_write_from_input; no plain write was dispatched."}
	}
	modelArgs := append(json.RawMessage(nil), args...)
	var source *execution.InputProof
	if name == execution.InputWriteTool {
		payload, proof, err := agents.ResolveInputWrite(task, args)
		if err != nil {
			return "", &agents.ToolFailure{Code: "execution_input_selection_invalid", Message: "The selected user input or unique boundaries are unavailable. No write was dispatched."}
		}
		args, _ = json.Marshal(payload)
		source = &proof
	}
	if name == "workspace_write" {
		var a struct {
			Path     string  `json:"path"`
			Content  *string `json:"content"`
			Revision string  `json:"revision"`
		}
		decoder := json.NewDecoder(bytes.NewReader(args))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&a) != nil || decoder.Decode(new(any)) != io.EOF || a.Content == nil || !utf8.ValidString(*a.Content) || len(*a.Content) > 131072 {
			return "", &agents.ToolFailure{Code: "execution_arguments_invalid", Message: "Provide a relative path, exact UTF-8 content up to 131072 bytes and an optional preimage revision. No write was dispatched."}
		}
		args, _ = json.Marshal(struct {
			Path     string `json:"path"`
			Data     []byte `json:"data"`
			Revision string `json:"revision"`
		}{a.Path, []byte(*a.Content), a.Revision})
	}
	if err := b.server.executionHosts.Admit(ctx, task.Actor, b.server.workspaceID, task.ID, *task.Config.Execution); err != nil {
		return "", &agents.ToolFailure{Code: "execution_admission_failed", Message: "The host could not admit this job. No operation was dispatched.", Cause: err}
	}
	binding := execution.Binding{Actor: task.Actor, Workspace: b.server.workspaceID, Job: task.ID, Host: task.Config.Execution.Host, Session: task.Config.Execution.Session, Grant: task.Config.Execution.Grant, Revision: task.Config.Execution.Revision}
	intent, err := execution.NewIntent(binding, e.CallID, executionOperation(name), args, e.Approved)
	if err != nil {
		return "", err
	}
	if execution.MutatingOperation(intent.Operation) {
		if err = b.server.agentManager.RecordExecutionIntent(task.ID, task.Actor, name, modelArgs, intent); err != nil {
			return "", err
		}
	}
	response, err := b.server.executionHosts.CallApproved(ctx, task.Actor, b.server.workspaceID, task.ID, e.CallID, *task.Config.Execution, executionOperation(name), args, e.Approved)
	if err != nil {
		return "", err
	} // uncertain external effects remain governed by the runner
	if execution.MutatingOperation(intent.Operation) && response.Error == "" && response.Receipt != nil {
		if err = b.server.agentManager.RecordExecutionReceipt(task.ID, task.Actor, intent, response); err != nil {
			return "", err
		}
	}
	if response.Receipt == nil {
		return "", &agents.ToolFailure{Code: "execution_rejected", Message: "The execution host rejected this operation before dispatch."}
	}
	if response.Receipt.State == "uncertain" || response.Receipt.State == "partial" {
		return "", fmt.Errorf("execution outcome %s; inspect host receipts before reconciliation", response.Receipt.State)
	}
	if response.Receipt.State == "rejected" {
		if execution.FileReadTool(name) {
			return "", &agents.ToolFailure{Code: "execution_rejected", Message: "The selected workspace read was rejected; no file changes occurred.", Recovery: "Use observed paths and their types within this folder: workspace_list reads directories; workspace_read reads files. Correct the read if the task needs it, or answer directly if no file observation is required. Do not substitute service files or invent a successful result."}
		}
		var result struct {
			Error string `json:"error"`
		}
		if executionOperation(name) == "file_write" && json.Unmarshal(response.Receipt.Result, &result) == nil && (result.Error == "execution_target_exists" || result.Error == "execution_revision_changed") {
			return "", &agents.ToolFailure{Code: "execution_write_precondition", Message: "The host rejected this write before changing the file: its existing content needs a current preimage revision.", Recovery: "Read this target with workspace_read now. Use its exact revision for the requested replacement, preserving the requested content. Do not repeat earlier completed writes or change another file. If the observed content conflicts with the user's request, ask for clarification."}
		}
		if name == "workspace_process_start" && json.Unmarshal(response.Receipt.Result, &result) == nil && result.Error == "execution_program_unavailable" {
			return "", &agents.ToolFailure{Code: "execution_program_unavailable", Message: "The selected host could not resolve this executable safely on its PATH. No command started. Use an installed host command or correct that host's environment."}
		}
		return "", &agents.ToolFailure{Code: "execution_rejected", Message: "The host rejected this operation. Review the relative path, file revision and grant before proposing another action."}
	}
	// Keep metadata and raw evidence explicit. A host receipt is not proof that
	// every semantic requirement in the user's request has been satisfied.
	if source != nil {
		return execution.PresentInputWrite(*task.Config.Execution, intent, *response.Receipt, *source)
	}
	return execution.PresentReceipt(*task.Config.Execution, executionOperation(name), *response.Receipt)
}

func (b *browserRunTools) workspaceTools(ctx context.Context, task *agents.Task) []api.Tool {
	if task.Config.Execution == nil || b.server.executionHosts == nil || task.ParentID != "" || !task.Config.TaskFirst {
		return nil
	}
	resource, err := b.server.executionHosts.Resolve(ctx, task.Actor, b.server.workspaceID, *task.Config.Execution)
	if err != nil || !executionScopeMatches(task, resource) {
		return nil
	}
	result := []api.Tool{}
	for _, tool := range executionTools() {
		if tool.Function.Name == execution.InputWriteTool {
			ids := agents.InputWriteSources(task)
			if len(ids) == 0 {
				continue
			}
			tool.Function.Parameters["properties"].(map[string]any)["input"].(map[string]any)["enum"] = ids
		}
		if task.Config.ExecutionPolicy != nil && !executionPermitted(tool.Function.Name, *task.Config.ExecutionPolicy) {
			continue
		}
		d, _ := executionDescriptor(tool.Function.Name)
		if executionPermitted(tool.Function.Name, resource.Permissions) && executionPermitted(tool.Function.Name, task.Config.ExecutionWorkspace.Permissions) && agents.ProfileAllowsCapability(task.Config, d.Name, d) && b.server.profileActorCapabilityAllowed(task.Actor, d) {
			result = append(result, workspacePageTool(tool, task))
		}
	}
	return result
}

func serviceFileTool(name string) bool {
	return name == "read_file" || name == "list_files" || name == "write_file"
}

func executionToolNamespace(name string) bool { return strings.HasPrefix(name, "workspace_") }

func executionScopeMatches(task *agents.Task, resource execution.Resource) bool {
	pinned := task.Config.ExecutionWorkspace
	return pinned != nil && pinned.Reference == resource.Reference && pinned.Root == resource.Root && pinned.Backend == resource.Backend
}
