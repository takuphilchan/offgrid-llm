package agents

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type WorkerIntegrationObservation struct {
	Key, RootID, Actor, WorkerID string
	Request                      execution.IntegrateChangeSetRequest
}

// A bounded cursor spans unresolved attempts, including stopped work. Reading
// receipts never restores grants, dispatches work, or changes a task's outcome.
func (m *Manager) WorkerIntegrationObservations(after string, limit int) []WorkerIntegrationObservation {
	if limit < 1 || limit > 64 {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var items []WorkerIntegrationObservation
	for _, root := range m.tasks {
		if root.ParentID != "" || root.Status == TaskRunning || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil {
			continue
		}
		for _, entry := range root.Checkpoint.StorageBudget.Reservations {
			i := entry.Integration
			if i == nil || i.State != "requested" && i.State != "accepted" && i.State != "uncertain" {
				continue
			}
			key := root.ID + "/" + entry.Request.WorkerRunID
			if key > after {
				items = append(items, WorkerIntegrationObservation{Key: key, RootID: root.ID, Actor: root.Actor, WorkerID: entry.Request.WorkerRunID, Request: i.Request})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items[:min(limit, len(items))]
}

func pendingWorkerIntegration(t *Task) (string, *WorkerIntegrationReservation) {
	if t == nil || t.ParentID != "" || t.WorkID == "" || t.Checkpoint == nil || t.Checkpoint.StorageBudget == nil {
		return "", nil
	}
	cp := t.Checkpoint
	d, _ := execution.Descriptor(execution.IntegrateTool)
	if cp.ExecutingCall == "" || cp.ExecutingCapability == nil || *cp.ExecutingCapability != d || len(cp.Calls) != 1 || cp.Calls[0].ID != cp.ExecutingCall || cp.Calls[0].Function.Name != execution.IntegrateTool {
		return "", nil
	}
	var args struct {
		WorkerRunID string `json:"worker_run_id"`
	}
	if decodeRuntimeArguments(cp.Calls[0].Function.Arguments, &args) != nil {
		return "", nil
	}
	for _, entry := range cp.StorageBudget.Reservations {
		i := entry.Integration
		if entry.Request.WorkerRunID == args.WorkerRunID && i != nil && i.CallID == cp.ExecutingCall && i.Request.Source.Actor == t.Actor && i.Request.Source.Job == t.ID {
			return args.WorkerRunID, i
		}
	}
	return "", nil
}

func recoverWorkerIntegrationReceipt(t *Task) bool {
	if t == nil || t.Status != TaskUncertain || t.ControlPending || t.ControlAction == "stop" {
		return false
	}
	_, i := pendingWorkerIntegration(t)
	if i == nil || i.Result == nil || i.State == "uncertain" || i.State == "accepted" || i.State == "requested" {
		return false
	}
	cp := t.Checkpoint
	data, err := json.Marshal(map[string]any{"integration": i.Result, "historical": true, "semantic_completion_verified": false, "guidance": "Recovered saved native file receipts without replay. Inspect current files and explicitly continue; this is not proof of task completion."})
	if err != nil {
		return false
	}
	result := string(data)
	t.Steps = append(t.Steps, Step{ID: len(t.Steps) + 1, CallID: i.CallID, Type: "action", Provenance: "execution_receipt", ToolName: execution.IntegrateTool, ToolArgs: cp.Calls[0].Function.Arguments, ToolResult: result, Timestamp: time.Now().UTC()})
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: i.CallID, Name: execution.IntegrateTool, Content: result})
	cp.Calls, cp.ExecutingCall, cp.ExecutingCapability = nil, "", nil
	t.Status, t.ErrorCode, t.Error, t.CompletedAt = TaskInterrupted, "execution_result_recovered", "Saved integration receipts recovered without repeating file operations. Review and continue explicitly.", nil
	return true
}
