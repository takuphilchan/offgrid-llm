package agents

import (
	"database/sql"
	"time"
)

// Ending an assignment must resolve accepted input without pretending it was
// applied. Keep its original run identity, contents and delivery receipt. A
// continuation starts a new assignment; it never silently replays this input.
func abandonEndedWorkerMessages(tx *sql.Tx, scope ControlScope, task *Task) (bool, error) {
	if !terminalControlTask(task) && task.ControlAction != "stop" {
		return false, nil
	}
	changed := false
	for _, entry := range task.PendingInstructions {
		if !task.WorkerMessageRequests[entry.RequestID] {
			continue
		}
		message, err := readWorkerMessage(tx, scope, task.Worker.SessionID, entry.RequestID)
		if err != nil {
			return false, err
		}
		if message.State == "applied" || message.State == "abandoned" {
			continue
		}
		if message.RunID != task.ID || message.State != "accepted" && message.State != "delivered" {
			return false, ErrRunConflict
		}
		now := time.Now().UTC()
		message.State, message.AbandonedAt = "abandoned", &now
		message.ResolutionReason = "assignment_ended_before_application"
		if task.ControlAction == "stop" {
			message.ResolutionReason = "stopped_before_application"
		}
		if err = putWorkerMessage(tx, scope, message); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func checkWorkerInboxSettled(q controlReader, scope ControlScope, task *Task) error {
	for _, entry := range task.PendingInstructions {
		if !task.WorkerMessageRequests[entry.RequestID] {
			return ErrRunConflict
		}
		message, err := readWorkerMessage(q, scope, task.Worker.SessionID, entry.RequestID)
		if err != nil {
			return err
		}
		if message.State != "applied" && message.State != "abandoned" {
			return ErrRunConflict
		}
	}
	return nil
}
