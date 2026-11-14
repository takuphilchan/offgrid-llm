package agents

import (
	"database/sql"
	"encoding/json"
)

// AdmitConversationWork uses the existing runner, including its recovery,
// permissions and reservations. It does not introduce another executor.
func (r *Runner) AdmitConversationWork(scope ControlScope, c ConversationCommand, model string, config AgentConfig) (*ConversationCommandReceipt, error) {
	if c.Kind != "create_work" || c.Payload.Input == nil {
		return nil, ErrControlCommand
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrRunConflict
	}
	if receipt, err := r.manager.ConversationCommandReceipt(scope, c); err != nil || receipt != nil {
		return receipt, err
	}
	origin := &ConversationOrigin{Workspace: scope.Workspace, Conversation: c.TargetID, TurnID: c.RequestID}
	task, err := prepareConversationRun(c.Payload.Input.Content, model, scope.Actor, config, origin)
	if err != nil {
		return nil, err
	}
	task.RequestID = "control-" + runID()
	task.RequestDigest = commandDigest(c)
	// r.mu serializes runner admission. SQLite serializes configuration writes.
	receipt, err := r.manager.commitConversationCommand(scope, c, task)
	if err != nil {
		return nil, err
	}
	if receipt.Work == nil {
		return nil, ErrRunStorage
	}
	if receipt.Work.RunID == task.ID {
		r.manager.mu.Lock()
		r.manager.tasks[task.ID] = task
		r.manager.mu.Unlock()
	}
	return receipt, nil
}

// Called inside the task's own transaction. Work state and the task receipt
// cannot disagree after a crash, even if an observer or JSON projection fails.
func syncControlWork(tx *sql.Tx, task *Task) error {
	if task.WorkID == "" {
		return nil
	}
	if task.Origin == nil || task.ParentID != "" {
		return ErrRunStorage
	}
	scope := ControlScope{task.Origin.Workspace, task.Actor}
	r, err := readControlRecord(tx, scope, task.WorkID, "work")
	if err != nil {
		return err
	}
	var work WorkItem
	if err = json.Unmarshal(r.Snapshot, &work); err != nil {
		return err
	}
	if work.RunID != task.ID {
		// Historical terminal runs may still acquire archive metadata. They
		// cannot publish into the current work attempt or become executable.
		for _, id := range work.RunIDs {
			if id == task.ID && terminalControlTask(task) {
				return nil
			}
		}
		return ErrRunConflict
	}
	ref := ""
	if task.Result != "" && (task.Status == TaskCompleted || task.Status == TaskFailed || task.Status == TaskCancelled) {
		ref = resultReference(task)
	}
	status := task.Status
	if hasConversationHold(task) && task.ControlAction == "" && !terminalControlTask(task) {
		status = TaskConversationInput
	}
	if task.ControlPending {
		if task.ControlAction == "stop" {
			status = "stopping"
		}
		if task.ControlAction == "pause" {
			status = "pausing"
		}
	}
	changed := work.Status != status || work.InstructionVersion != task.InstructionVersion || work.ControlEpoch != task.ControlEpoch || work.ResultReference != ref
	if changed {
		work.Revision++
		work.Status = status
		work.InstructionVersion = task.InstructionVersion
		work.ControlEpoch = task.ControlEpoch
		work.ResultReference = ref
		if err = putControlRecord(tx, scope, work.ID, "work", work.Revision, &work); err != nil {
			return err
		}
		if _, err = appendControlEvent(tx, scope, work.ID, work.Revision, "work_state", &work, ""); err != nil {
			return err
		}
	}
	if task.StartedAt != nil || task.Status != TaskPending {
		_, err = tx.Exec(`UPDATE agent_control_outbox SET disposition=? WHERE run_id=? AND disposition='pending'`, string(task.Status), task.ID)
	}
	return err
}
