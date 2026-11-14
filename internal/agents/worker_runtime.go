package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workerRuntimeTool(name string) bool {
	return name == "worker_inspect" || name == "worker_message" || name == "worker_continue" || name == "worker_close"
}

func workerRuntimeTools(task *Task) []api.Tool {
	if task.WorkID == "" || task.ParentID != "" || task.Origin == nil || workerSessionCount(task) == 0 {
		return nil
	}
	result := []api.Tool{}
	for _, operation := range []struct{ name, description string }{
		{"worker_inspect", "Inspect this work's resumable workers and result references. Omit worker_id to list them. This does not run, resume, or wait for inference."},
		{"worker_message", "Send a bounded follow-up to an active worker. The message is applied once at a checkpoint and does not replenish its allowance."},
		{"worker_continue", "Continue a completed worker under its existing session with retained context and the root's remaining allowance. Cannot revive a user-stopped worker. The parent waits without holding inference capacity."},
		{"worker_close", "Close a settled worker session that is no longer needed. Its evidence and run history remain available."},
	} {
		if task.Config.DisableDelegation && (operation.name == "worker_message" || operation.name == "worker_continue") {
			continue
		}
		properties := map[string]interface{}{"worker_id": map[string]interface{}{"type": "string"}}
		required := []string{}
		if operation.name != "worker_inspect" {
			required = []string{"worker_id", "expected_revision"}
			properties["expected_revision"] = map[string]interface{}{"type": "integer", "minimum": 1}
		}
		if operation.name == "worker_message" || operation.name == "worker_continue" {
			properties["content"] = map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 16000}
			required = append(required, "content")
		}
		result = append(result, api.Tool{Type: "function", Function: api.FunctionDef{Name: operation.name, Description: operation.description, Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": required, "properties": properties}}})
	}
	return result
}

func (r *Runner) executeWorkerRuntime(ctx context.Context, task *Task, call api.ToolCall) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if task.WorkID == "" || task.ParentID != "" || task.Origin == nil {
		return "", ErrRunConflict
	}
	var args struct {
		WorkerID         string `json:"worker_id"`
		ExpectedRevision int64  `json:"expected_revision"`
		Content          string `json:"content"`
	}
	if err := decodeRuntimeArguments(call.Function.Arguments, &args); err != nil {
		return "", err
	}
	scope := ControlScope{task.Origin.Workspace, task.Actor}
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range task.PreviousWorkerIDs {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, link := range task.ChildHistory {
		if link.WorkerID != "" && !seen[link.WorkerID] {
			ids = append(ids, link.WorkerID)
			seen[link.WorkerID] = true
		}
	}
	if args.WorkerID != "" && !seen[args.WorkerID] {
		return "", ErrControlNotFound
	}
	if call.Function.Name == "worker_inspect" {
		workers := []api.WorkerSession{}
		for _, id := range ids {
			if args.WorkerID != "" && id != args.WorkerID {
				continue
			}
			worker, err := r.manager.WorkerSession(scope, id)
			if err != nil {
				return "", err
			}
			current, exists := r.manager.GetTask(worker.CurrentRunID)
			if !exists || r.checkSourceAccess(current) != nil {
				continue
			}
			workers = append(workers, *worker)
		}
		data, err := json.Marshal(map[string]any{"workers": workers, "independently_verified": false})
		return string(data), err
	}
	if args.WorkerID == "" {
		return "", ErrControlCommand
	}
	worker, err := r.manager.WorkerSession(scope, args.WorkerID)
	if err != nil {
		return "", err
	}
	current, exists := r.manager.GetTask(worker.CurrentRunID)
	if !exists || worker.WorkID != task.WorkID || r.checkSourceAccess(current) != nil {
		return "", ErrControlNotFound
	}
	kind := strings.TrimPrefix(call.Function.Name, "worker_")
	if task.Config.DisableDelegation && kind != "close" {
		return "", ErrRunConflict
	}
	hash := sha256.Sum256([]byte(task.ID + "\x00" + call.ID))
	command := api.WorkerCommand{RequestID: "worker-tool-" + hex.EncodeToString(hash[:]), TargetID: worker.ID, ExpectedRevision: args.ExpectedRevision, Kind: kind, Payload: api.WorkerCommandPayload{Content: args.Content}}
	if err = validateWorkerCommand(command); err != nil {
		return "", err
	}
	var receipt *ConversationCommandReceipt
	if kind == "continue" {
		receipt, err = r.continueWorker(scope, command, false)
		if err == nil {
			updated, exists := r.manager.GetTask(task.ID)
			if !exists {
				return "", ErrTaskNotFound
			}
			*task = *updated
		}
	} else {
		receipt, err = r.CommandWorker(scope, command)
	}
	if err != nil {
		return "", fmt.Errorf("worker command unavailable: %w", err)
	}
	data, err := json.Marshal(receipt)
	return string(data), err
}
