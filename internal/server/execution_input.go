package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workspaceAccessTool() api.Tool {
	return execution.AccessRequestSchema()
}

func requestableWorkspace(task *agents.Task) bool {
	if !task.Config.TaskFirst || task.Config.TextOnlyInference || task.ParentID != "" || task.Config.Automation != nil || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion {
		return false
	}
	ceiling := execution.Permissions{Read: true, Write: true, Processes: true}
	if task.Config.ExecutionPolicy != nil {
		ceiling = *task.Config.ExecutionPolicy
	}
	if task.Config.Profile != nil && task.Config.Profile.Settings.Execution != nil {
		p := task.Config.Profile.Settings.Execution
		ceiling.Read = ceiling.Read && p.Read
		ceiling.Write = ceiling.Write && p.Write
		ceiling.Processes = ceiling.Processes && p.Processes
	}
	if !ceiling.Read {
		return false
	}
	return task.Config.Execution == nil || task.Config.ExecutionWorkspace != nil && !execution.PermissionsWithin(ceiling, task.Config.ExecutionWorkspace.Permissions)
}

func workspaceAccessRequest(task *agents.Task, args json.RawMessage, e agents.ToolExecution) error {
	d := execution.AccessRequestDescriptor()
	if !requestableWorkspace(task) || task.Actor != e.Actor || task.ID != e.RunID || e.ExpectedCapability == nil || *e.ExpectedCapability != d {
		return capabilities.ErrDenied
	}
	var fields map[string]json.RawMessage
	var mode string
	if jsonwire.Validate(args) != nil || json.Unmarshal(args, &fields) != nil || len(fields) != 1 || json.Unmarshal(fields["access"], &mode) != nil {
		return capabilities.ErrDenied
	}
	want, ok := execution.AccessPermissions(mode)
	if task.Config.ExecutionWorkspace != nil && execution.PermissionsWithin(want, task.Config.ExecutionWorkspace.Permissions) {
		return capabilities.ErrDenied
	}
	if !ok || task.Config.ExecutionPolicy != nil && !execution.PermissionsWithin(want, *task.Config.ExecutionPolicy) || task.Config.Profile != nil && task.Config.Profile.Settings.Execution != nil && !execution.PermissionsWithin(want, *task.Config.Profile.Settings.Execution) {
		return capabilities.ErrDenied
	}
	return &agents.InputRequired{Request: agents.InputRequest{Kind: "workspace", Mode: mode, Target: "Local workspace"}}
}

func (b *browserRunTools) authorizeWorkspaceInput(task *agents.Task, args json.RawMessage, e agents.ToolExecution) error {
	if task.Status != agents.TaskRunning || !b.server.profileActorCapabilityAllowed(task.Actor, execution.AccessRequestDescriptor()) {
		return capabilities.ErrDenied
	}
	return workspaceAccessRequest(task, args, e)
}

func (s *Server) handleExecutionInput(w http.ResponseWriter, r *http.Request, task *agents.Task, inputID string, ref execution.Reference) {
	if s.executionHosts == nil || ref.Validate() != nil || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion || task.ParentID != "" || task.AdmissionHeld || task.Config.Automation != nil {
		writeJobError(w, 409, "workspace_input_unavailable", "This job cannot attach that workspace. No operation was dispatched.", false)
		return
	}
	// A lost successful reply observes the same attachment; no second admission.
	duplicate := task.ResolvedInputID == inputID && task.Config.Execution != nil && *task.Config.Execution == ref
	if !duplicate && (task.Status != agents.TaskInput || task.PendingInput == nil || task.PendingInput.ID != inputID || task.PendingInput.Kind != "workspace") {
		writeJobError(w, 409, "task_input_changed", "This job no longer needs that workspace request. Inspect saved state.", false)
		return
	}
	resource, err := s.executionHosts.Resolve(r.Context(), task.Actor, s.workspaceID, ref)
	if err != nil {
		writeJobError(w, 409, "execution_unavailable", "The locally approved workspace is unavailable. No operation was dispatched.", true)
		return
	}
	if !duplicate {
		// The input event is durable before the worker finishes teardown. A fast
		// local approval must not fail merely because that last unwind is pending.
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for s.agentRunner.WorkerActive(task.ID) {
			select {
			case <-r.Context().Done():
				writeJobCommandError(w, agents.ErrRunConflict)
				return
			case <-deadline.C:
				writeJobCommandError(w, agents.ErrRunConflict)
				return
			case <-tick.C:
			}
		}
		task, err = s.agentRunner.ResolveExecutionInput(task.ID, task.Actor, inputID, resource)
		if err != nil {
			writeJobCommandError(w, err)
			return
		}
	}
	if task.Status == agents.TaskPending {
		task, err = s.agentRunner.Continue(s.agentContext(r, true), task.ID, task.Actor, "start", "", true)
		if err != nil {
			writeAgentRunError(w, task, err)
			return
		}
	}
	writeJSON(w, http.StatusAccepted, taskResponse(task))
}
