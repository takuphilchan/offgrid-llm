package agents

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Runs are immutable completed attempts. A worker follow-up can advance the
// work's current run, but it must carry every previously reserved budget unit.
// Callers hold runner/manager locks and commit this with the worker command.
func (r *Runner) prepareWorkerWorkAttempt(tx *sql.Tx, scope ControlScope, worker *api.WorkerSession, parent *Task, explicitUser bool) (*Task, error) {
	record, err := readControlRecord(tx, scope, worker.WorkID, "work")
	if err != nil {
		return nil, err
	}
	var work WorkItem
	if err = json.Unmarshal(record.Snapshot, &work); err != nil {
		return nil, err
	}
	current := r.manager.tasks[work.RunID]
	if current == nil || current.Actor != scope.Actor || current.WorkID != worker.WorkID || current.Origin == nil || current.Origin.Workspace != scope.Workspace || current.DeletedAt != nil {
		return nil, ErrControlNotFound
	}
	if current.InstructionVersion != parent.InstructionVersion {
		return nil, ErrRunConflict
	}
	if !terminalControlTask(current) {
		if current.Status != TaskChildren && current.Status != TaskRunning && current.Status != TaskPending || current.Checkpoint == nil || current.Checkpoint.ExecutingCall != "" || current.ControlPending || current.ControlAction == "stop" || hasConversationHold(current) || len(current.PendingInstructions) > 0 {
			return nil, ErrRunConflict
		}
		copy, err := copyTask(current)
		if err == nil {
			r.settleChildBudgetsLocked(copy)
		}
		return copy, err
	}
	if !explicitUser || !terminalControlTask(current) || current.ControlAction == "stop" || current.Status == TaskCancelled || r.active[current.ID] != nil || current.Checkpoint == nil || current.Checkpoint.ExecutingCall != "" || current.ProcessStopPending || hasConversationHold(current) || len(current.PendingInstructions) > 0 {
		return nil, ErrRunConflict
	}
	for _, link := range current.ChildHistory {
		child := r.manager.tasks[link.ID]
		if child == nil || !terminalControlTask(child) || r.active[child.ID] != nil || child.ProcessStopPending || child.Checkpoint == nil || child.Checkpoint.ExecutingCall != "" {
			return nil, ErrRunConflict
		}
	}
	copy, err := copyTask(current)
	if err != nil {
		return nil, err
	}
	r.settleChildBudgetsLocked(copy)
	// Budget feasibility is checked before changing the work's current identity.
	if _, err = childDecisionBudget(copy, 1); err != nil {
		return nil, err
	}
	cp := copy.Checkpoint
	for _, call := range cp.Calls {
		cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: "Previous work attempt ended; this proposal was not dispatched in the follow-up."})
	}
	cp.Messages = appendRetainedOutcome(cp.Messages, copy.Result)
	// Retain only context and cumulative resource charges. Receipts and executable
	// proposals belong to the old run and remain there for inspection.
	checkpoint := &Checkpoint{Messages: cp.Messages, Iteration: cp.Iteration, BudgetedModelCalls: cp.BudgetedModelCalls, ReservedChildIterations: cp.ReservedChildIterations, ChargedCompletionTokens: cp.ChargedCompletionTokens, ChargedInferenceNS: cp.ChargedInferenceNS, MeasuredCompletionTokens: cp.MeasuredCompletionTokens, MeasuredInferenceNS: cp.MeasuredInferenceNS, UnmeasuredModelCalls: cp.UnmeasuredModelCalls, SyntheticProbeCalls: cp.SyntheticProbeCalls, SyntheticProbeTokens: cp.SyntheticProbeTokens, SyntheticProbeNS: cp.SyntheticProbeNS}
	checkpoint.ChildBudgetSettlements = cp.ChildBudgetSettlements
	checkpoint.InheritedChildIterations = cp.ReservedChildIterations
	checkpoint.InputBudget = cp.InputBudget
	checkpoint.ToolBudget = cp.ToolBudget
	checkpoint.ProcessBudget = cp.ProcessBudget
	checkpoint.StorageBudget = cp.StorageBudget
	checkpoint.ContextPreparations = cp.ContextPreparations
	checkpoint.MeasuredContextPreparationNS = cp.MeasuredContextPreparationNS
	config := copy.Config
	// Retain the selected source as a preference only. The controller must
	// resolve its live host authority before binding the new root attempt.
	checkpoint.WorkerSource = config.ExecutionWorkspace
	if checkpoint.WorkerSource == nil {
		checkpoint.WorkerSource = cp.WorkerSource
	}
	// A saved run is not a new execution grant. The ordinary capability request
	// path must supply a run-bound binding when new execution is needed.
	config.Execution, config.ExecutionWorkspace = nil, nil
	config.ComputerSession, config.ComputerDriver, config.ComputerScope, config.ComputerApprovalMode = "", "", "", ""
	config.ComputerStepOffset = 0
	previous := append([]string{}, copy.PreviousWorkerIDs...)
	seen := map[string]bool{}
	for _, id := range previous {
		seen[id] = true
	}
	for _, child := range copy.ChildHistory {
		if child.WorkerID != "" && !seen[child.WorkerID] {
			previous = append(previous, child.WorkerID)
			seen[child.WorkerID] = true
		}
	}
	next := &Task{ID: "run-" + runID(), WorkID: current.WorkID, PreviousWorkerIDs: previous, Actor: current.Actor, Model: current.Model, Prompt: current.Prompt, Status: TaskChildren, Config: config, Origin: copy.Origin, CreatedAt: time.Now().UTC(), InstructionVersion: current.InstructionVersion, ControlEpoch: current.ControlEpoch + 1, Checkpoint: checkpoint, Context: copy.Context, ContextArchive: copy.ContextArchive}
	if len(work.RunIDs) == 0 {
		work.RunIDs = []string{work.RunID}
	}
	work.RunIDs = append(work.RunIDs, next.ID)
	work.RunID, work.Status, work.ResultReference, work.IndependentlyVerified = next.ID, TaskChildren, "", false
	work.Revision++
	if err = putControlRecord(tx, scope, work.ID, "work", work.Revision, &work); err != nil {
		return nil, err
	}
	return next, nil
}

func appendRetainedOutcome(messages []api.ChatMessage, result string) []api.ChatMessage {
	if result == "" {
		return messages
	}
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		if last.Role == "assistant" && len(last.ToolCalls) == 0 && last.StringContent() == result {
			return messages
		}
	}
	return append(messages, api.ChatMessage{Role: "assistant", Content: result})
}
