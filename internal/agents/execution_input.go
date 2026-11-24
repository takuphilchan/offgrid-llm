package agents

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ResolveExecutionInput attaches validated local authority at the paused root.
// It does not dispatch the interrupted request or replace any other job state.
func (r *Runner) ResolveExecutionInput(id, actor, inputID string, resource execution.Resource) (*Task, error) {
	return r.ResolveExecutionInputWithProject(id, actor, inputID, resource, nil)
}

// Project discovery is optional for legacy hosts. When supplied, its binding
// and the accepted access input share one durable transaction.
func (r *Runner) ResolveExecutionInputWithProject(id, actor, inputID string, resource execution.Resource, project *ExecutionProjectContext) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing || inputID == "" || resource.Reference.Validate() != nil {
		return nil, ErrRunConflict
	}
	if _, active := r.active[id]; active {
		return nil, ErrRunConflict
	}
	change := func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if task.AdmissionHeld || task.ParentID != "" || task.Config.ExecutionRequestVersion != execution.AccessRequestVersion || task.Config.Automation != nil {
			return ErrRunConflict
		}
		if task.ResolvedInputID == inputID && task.Config.Execution != nil && *task.Config.Execution == resource.Reference {
			return nil
		}
		if hasConversationHold(task) {
			return ErrRunConflict
		}
		cp, input := task.Checkpoint, task.PendingInput
		if task.Status != TaskInput || input == nil || input.ID != inputID || input.Kind != "workspace" || cp == nil || cp.ExecutingCall != "" || cp.ExecutionIntent != nil || len(cp.Calls) != 1 || cp.Calls[0].ID != input.CallID || cp.Calls[0].Function.Name != execution.AccessRequestTool {
			return ErrRunConflict
		}
		want, ok := execution.AccessPermissions(input.Mode)
		if old := task.Config.ExecutionWorkspace; old != nil {
			if old.Root != resource.Root || old.Reference.Host != resource.Reference.Host || old.Backend != resource.Backend || execution.PermissionsWithin(want, old.Permissions) || !execution.PermissionsWithin(old.Permissions, resource.Permissions) {
				return ErrRunConflict
			}
		}
		if !ok || !execution.PermissionsWithin(want, resource.Permissions) || task.Config.ExecutionPolicy != nil && !execution.PermissionsWithin(resource.Permissions, *task.Config.ExecutionPolicy) {
			return ErrRunConflict
		}
		if task.Config.Profile != nil && task.Config.Profile.Settings.Execution != nil && !execution.PermissionsWithin(resource.Permissions, *task.Config.Profile.Settings.Execution) {
			return ErrRunConflict
		}
		config := task.Config
		ref := resource.Reference
		config.Execution, config.ExecutionWorkspace, config.InputWriteVersion = &ref, &resource, execution.InputWriteVersion
		// Clone the map: rejected persistence must not change live authority.
		config.AllowedCapabilities = make(map[string]capabilities.Descriptor, len(task.Config.AllowedCapabilities)+16)
		for name, d := range task.Config.AllowedCapabilities {
			config.AllowedCapabilities[name] = d
		}
		for _, tool := range execution.Tools() {
			if execution.Permitted(tool.Function.Name, resource.Permissions) {
				d, _ := execution.Descriptor(tool.Function.Name)
				if executionInputCapabilityAllowed(config, d) {
					config.AllowedCapabilities[d.Name] = d
				}
			}
		}
		if d := execution.AccessRequestDescriptor(); executionInputCapabilityAllowed(config, d) {
			config.AllowedCapabilities[d.Name] = d
		}
		if err := config.validateExecution(); err != nil {
			return err
		}
		result, _ := json.Marshal(map[string]any{"status": "workspace_attached", "host": ref.Host, "permissions": resource.Permissions, "requested_access": input.Mode, "operation_executed": false, "execution": ref, "previous_execution": task.Config.Execution,
			"text": "Access is ready; no file or command operation ran. Continue the requested work using the available tools, not instructions for the user to call them. If this is a conversational request, answer directly."})
		call := cp.Calls[0]
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: string(result)})
		cp.Calls = nil
		task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "input_resolved", ToolName: call.Function.Name, ToolResult: string(result), Timestamp: time.Now().UTC()})
		task.Config, task.PendingInput, task.ResolvedInputID = config, nil, inputID
		task.Status, task.Error, task.ErrorCode = TaskPending, "", ""
		return nil
	}
	var task *Task
	var err error
	if project == nil {
		task, err = r.manager.updateTask(id, func(task *Task) error {
			duplicate := task.Actor == actor && task.ResolvedInputID == inputID && task.Config.Execution != nil && *task.Config.Execution == resource.Reference
			if err := change(task); err != nil {
				return err
			}
			if duplicate {
				return nil
			}
			return r.manager.acceptRootModelProgress(r.manager.tasks[id], task)
		})
	} else {
		task, err = r.manager.updateTaskControl(id, func(task *Task, tx *sql.Tx) error {
			duplicate := task.Actor == actor && task.ResolvedInputID == inputID && task.Config.Execution != nil && *task.Config.Execution == resource.Reference
			if err := change(task); err != nil {
				return err
			}
			if duplicate {
				return nil
			}
			if err := bindExecutionProject(tx, task, resource, project); err != nil {
				return err
			}
			return r.manager.acceptRootModelProgress(r.manager.tasks[id], task)
		})
	}
	if err == nil {
		r.observe(task)
	}
	return task, err
}

func executionInputCapabilityAllowed(config AgentConfig, d capabilities.Descriptor) bool {
	if config.ConversationConfiguration == nil || config.ConversationConfiguration.Restrictions.Tools == nil {
		return true
	}
	for _, ref := range *config.ConversationConfiguration.Restrictions.Tools {
		if ref.Provider == d.Source && ref.Tool == d.Name && ref.SchemaRevision == d.Revision && ref.Location == d.Location {
			return true
		}
	}
	return false
}
