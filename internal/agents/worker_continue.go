package agents

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Continue a settled assignment under the same session with a distinct run and
// allocation. The root keeps all prior charges. Model calls cannot revive a
// user-stopped worker; the public user command passes explicitUser=true.
func (r *Runner) continueWorker(scope ControlScope, c api.WorkerCommand, explicitUser bool) (*ConversationCommandReceipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrRunConflict
	}
	db, err := r.manager.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r.manager.mu.Lock()
	batch := []*Task{}
	cancelIDs := []string{}
	receipt, err := withControlCommandDatabase(db, scope, c.RequestID, workerCommandDigest(c), func(tx *sql.Tx) (*ConversationCommandReceipt, error) {
		record, err := readControlRecord(tx, scope, c.TargetID, "worker")
		if err != nil {
			return nil, err
		}
		if record.Revision != c.ExpectedRevision {
			return nil, ErrRunConflict
		}
		var worker api.WorkerSession
		if err = json.Unmarshal(record.Snapshot, &worker); err != nil {
			return nil, err
		}
		old, parent := r.manager.tasks[worker.CurrentRunID], r.manager.tasks[worker.RootRunID]
		if hasConversationHold(parent) || hasConversationHold(old) {
			return nil, ErrRunConflict
		}
		if worker.Closed || worker.Stopped && !explicitUser || old == nil || parent == nil || old.Actor != scope.Actor || parent.Actor != scope.Actor || parent.WorkID != worker.WorkID || old.Worker == nil || old.Worker.Workspace != scope.Workspace || old.Worker.SessionID != worker.ID || old.ParentID != parent.ID || old.DeletedAt != nil || parent.DeletedAt != nil {
			return nil, ErrRunConflict
		}
		if !terminalControlTask(old) || r.active[old.ID] != nil || old.Checkpoint == nil || old.Checkpoint.ExecutingCall != "" || parent.Checkpoint == nil || parent.Checkpoint.ExecutingCall != "" || len(parent.PendingInstructions) > 0 || parent.ControlPending || parent.ControlAction == "stop" || old.InstructionVersion != parent.InstructionVersion || len(parent.ChildHistory) >= childHistoryLimit(parent) {
			return nil, ErrRunConflict
		}
		attempt, err := r.prepareWorkerWorkAttempt(tx, scope, &worker, parent, explicitUser)
		if err != nil {
			return nil, err
		}
		budget, err := childDecisionBudget(attempt, 1)
		if err != nil {
			return nil, err
		}
		root, err := copyTask(attempt)
		if err != nil {
			return nil, err
		}
		newAttempt := r.manager.tasks[root.ID] == nil
		if newAttempt {
			root.Prompt, root.Origin.TurnID = c.Payload.Content, c.RequestID
			root.RequestID, root.RequestDigest = "control-"+runID(), workerCommandDigest(c)
			root.Checkpoint.Messages = append(root.Checkpoint.Messages, api.ChatMessage{Role: "user", Content: "Follow-up on retained worker: " + c.Payload.Content})
		}
		previous, err := copyTask(old)
		if err != nil {
			return nil, err
		}
		var spec ChildSpec
		found := false
		for _, link := range parent.ChildHistory {
			if link.ID == old.ID {
				spec, found = link.Spec, true
				break
			}
		}
		if !found {
			return nil, ErrRunStorage
		}
		spec.Goal = c.Payload.Content
		// A follow-up supplies a new objective. Do not present the previous
		// deliverable or delegation rationale as if it described this request.
		spec.ExpectedResult, spec.Reason = "", ""
		next := &Task{ID: "run-" + runID(), ParentID: root.ID, Actor: scope.Actor, Model: old.Model, Prompt: spec.Goal, Status: TaskPending, Config: previous.Config, CreatedAt: time.Now().UTC(), Worker: previous.Worker, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch + 1, AssignmentVersion: old.AssignmentVersion + 1, Checkpoint: &Checkpoint{Messages: previous.Checkpoint.Messages}, Context: previous.Context, ContextArchive: previous.ContextArchive}
		if previous.Checkpoint.WorkerEnvironment != nil {
			retained, err := retainedWorkerInput(parent, previous)
			if err != nil {
				return nil, err
			}
			next.Checkpoint.WorkerContinuation = retained
			// The sealed candidate includes earlier prerequisites. Do not rerun
			// them or replace that input with a later result under the same key.
			spec.DependsOn = nil
			// The previous native binding admits exactly the previous run. A
			// follow-up retains evidence, but requires a fresh narrowed binding.
			next.Config.Execution, next.Config.ExecutionWorkspace = nil, nil
		}
		next.Config.MaxIterations = budget
		next.DependencyResults = previous.DependencyResults
		// Close any old undelivered proposals in the retained context. Their
		// operations are never copied into the new run's executable checkpoint.
		for _, call := range previous.Checkpoint.Calls {
			next.Checkpoint.Messages = append(next.Checkpoint.Messages, api.ChatMessage{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: "Previous assignment ended. This proposal was not carried into the follow-up."})
		}
		next.Checkpoint.Messages = appendRetainedOutcome(next.Checkpoint.Messages, old.Result)
		next.Checkpoint.Messages = append(next.Checkpoint.Messages, workerAssignmentMessage(spec))
		link := ChildLink{ID: next.ID, WorkerID: worker.ID, Spec: spec, Status: TaskPending}
		root.ControlEpoch++
		root.Checkpoint.ReservedChildIterations += budget
		for i := range root.ChildHistory {
			if root.ChildHistory[i].ID == old.ID {
				root.ChildHistory[i].Invalidated = true
			}
		}
		root.ChildHistory = append(root.ChildHistory, link)
		batch = append(batch, root)
		if root.Delegation == nil {
			call := api.ToolCall{}
			if !explicitUser && len(root.Checkpoint.Calls) == 1 && root.Checkpoint.Calls[0].Function.Name == "worker_continue" {
				call = root.Checkpoint.Calls[0]
			} else {
				abandonProposals(root, "Not dispatched: a worker follow-up was accepted before this proposal.")
				call = api.ToolCall{ID: "call-" + runID(), Type: "function", Function: api.FunctionCall{Name: "worker_continue", Arguments: `{}`}}
				root.Checkpoint.Messages = append(root.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}})
			}
			root.Checkpoint.Calls = []api.ToolCall{call}
			root.Delegation = &Delegation{CallID: call.ID, Children: []ChildLink{link}}
			// Earlier prerequisites are retained as evidence, not rerun jobs in
			// this new graph. Their exact receipts remain in the worker context.
			root.Delegation.Children[0].Spec.DependsOn = nil
			root.ChildHistory[len(root.ChildHistory)-1].Spec.DependsOn = nil
		} else {
			replaced := false
			for i, existing := range root.Delegation.Children {
				if existing.ID == old.ID {
					root.Delegation.Children[i] = link
					replaced = true
					break
				}
			}
			if !replaced {
				if root.ID == parent.ID {
					return nil, ErrRunConflict
				}
				for _, existing := range root.Delegation.Children {
					if existing.Spec.Key == link.Spec.Key {
						return nil, ErrRunConflict
					}
				}
				link.Spec.DependsOn = nil
				root.Delegation.Children = append(root.Delegation.Children, link)
			}
			// Descendants prepared from the replaced result cannot remain valid.
			invalid := map[string]bool{spec.Key: true}
			for changed := true; changed; {
				changed = false
				for _, candidate := range root.Delegation.Children {
					if invalid[candidate.Spec.Key] {
						continue
					}
					for _, dependency := range candidate.Spec.DependsOn {
						if invalid[dependency] {
							invalid[candidate.Spec.Key] = true
							changed = true
							break
						}
					}
				}
			}
			for _, candidate := range root.Delegation.Children {
				if candidate.ID == next.ID || !invalid[candidate.Spec.Key] {
					continue
				}
				for i := range root.Delegation.Children {
					if root.Delegation.Children[i].ID == candidate.ID {
						root.Delegation.Children[i].Invalidated = true
					}
				}
				for i := range root.ChildHistory {
					if root.ChildHistory[i].ID == candidate.ID {
						root.ChildHistory[i].Invalidated = true
					}
				}
				dependent := r.manager.tasks[candidate.ID]
				if dependent == nil || dependent.Actor != scope.Actor || dependent.ParentID != root.ID {
					return nil, ErrRunStorage
				}
				if terminalControlTask(dependent) {
					continue
				}
				fenced, err := copyTask(dependent)
				if err != nil {
					return nil, err
				}
				if !terminalControlTask(fenced) {
					fenceWorkRun(fenced, "correct")
				}
				if fenced.Status != TaskUncertain {
					finishTask(fenced, TaskCancelled, "Prerequisite result changed. Retained output requires revalidation.")
					fenced.ErrorCode = "dependency_superseded"
				}
				batch = append(batch, fenced)
				cancelIDs = append(cancelIDs, fenced.ID)
			}
		}
		root.Status, root.SchedulerReady = TaskChildren, false
		root.ErrorCode, root.Error = "", ""
		worker.Revision++
		worker.RootRunID = root.ID
		worker.CurrentRunID, worker.Stopped, worker.Status = next.ID, false, TaskPending
		worker.RunIDs = append(worker.RunIDs, next.ID)
		worker.Assignment = workerAssignment(spec, next)
		worker.AssignmentVersion, worker.ResultReference = next.AssignmentVersion, ""
		if err = putControlRecord(tx, scope, worker.ID, "worker", worker.Revision, &worker); err != nil {
			return nil, err
		}
		batch = append(batch, next)
		for _, task := range batch {
			if err = putTaskTransaction(tx, task); err != nil {
				return nil, err
			}
		}
		if newAttempt {
			current, e := readControlRecord(tx, scope, worker.WorkID, "work")
			if e != nil {
				return nil, e
			}
			var work WorkItem
			if e = json.Unmarshal(current.Snapshot, &work); e != nil {
				return nil, e
			}
			if _, e = appendControlEvent(tx, scope, work.ID, work.Revision, "work_attempt_accepted", &work, root.ID); e != nil {
				return nil, e
			}
		}
		cursor, err := appendControlEvent(tx, scope, worker.ID, worker.Revision, "worker_continued", &worker, "")
		if err != nil {
			return nil, err
		}
		cancelIDs = append(cancelIDs, root.ID)
		return &ConversationCommandReceipt{RequestID: c.RequestID, TargetID: worker.ID, Revision: worker.Revision, EventCursor: cursor, Worker: &worker}, nil
	})
	if err == nil {
		for _, task := range batch {
			r.manager.tasks[task.ID] = task
		}
	}
	r.manager.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, id := range cancelIDs {
		if cancel := r.active[id]; cancel != nil {
			cancel()
		}
	}
	return receipt, nil
}
