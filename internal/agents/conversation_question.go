package agents

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Caller owns the runner/manager locks and command transaction. Answering a
// saved primary question is an explicit input operation, not a model turn.
func (r *Runner) acceptConversationQuestionAnswer(tx *sql.Tx, scope ControlScope, root *Task, entry *api.ConversationInboxEntry) ([]*Task, error) {
	if root.Status != TaskInput || !validWorkerClarificationInput(root) || root.PendingInput.ID != entry.Input.QuestionID || root.ControlAction != "" || root.ControlPending || hasConversationHold(root) || len(root.PendingInstructions) != 0 {
		return nil, ErrRunConflict
	}
	review := root.Delegation.QuestionReview
	child := r.manager.tasks[review.RunID]
	if child == nil || child.Actor != scope.Actor || child.ParentID != root.ID || child.DeletedAt != nil || child.Worker == nil || child.Worker.Workspace != scope.Workspace || child.Worker.WorkID != root.WorkID || child.Worker.SessionID != review.WorkerID {
		return nil, ErrControlNotFound
	}
	if child.ControlAction != "" || child.ControlPending || hasConversationHold(child) || child.InstructionVersion != root.InstructionVersion || len(child.Instructions)+len(child.PendingInstructions) >= 32 {
		return nil, ErrRunConflict
	}
	record, err := readControlRecord(tx, scope, review.WorkerID, "worker")
	if err != nil {
		return nil, err
	}
	var worker api.WorkerSession
	if err = json.Unmarshal(record.Snapshot, &worker); err != nil {
		return nil, err
	}
	if worker.Stopped || worker.Closed || worker.CurrentRunID != child.ID || worker.RootRunID != root.ID {
		return nil, ErrRunConflict
	}
	next, err := copyTask(child)
	if err != nil {
		return nil, err
	}
	command := api.WorkerCommand{RequestID: entry.ID, TargetID: worker.ID, ExpectedRevision: record.Revision, Kind: "answer", Payload: api.WorkerCommandPayload{QuestionID: entry.Input.QuestionID, Content: entry.Input.Content}}
	if err = answerWorkerQuestion(next, root, command); err != nil {
		return nil, err
	}
	message, err := appendWorkerMailboxInstruction(tx, scope, next, root, command)
	if err != nil {
		return nil, err
	}
	parent, err := copyTask(root)
	if err != nil {
		return nil, err
	}
	recordWorkerUserAnswer(parent, entry.ID, entry.Input.Content)
	finishWorkerQuestionReview(parent, root.Checkpoint.Calls[0].ID)
	parent.ControlEpoch++
	for _, task := range []*Task{parent, next} {
		if err = putTaskTransaction(tx, task); err != nil {
			return nil, err
		}
	}
	updated, err := readControlRecord(tx, scope, worker.ID, "worker")
	if err != nil {
		return nil, err
	}
	if _, err = appendControlEvent(tx, scope, worker.ID, updated.Revision, "worker_answer", message, ""); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	entry.Question = &api.ConversationQuestion{ID: root.PendingInput.ID, WorkID: root.WorkID, RunID: root.ID, Content: root.PendingInput.Target}
	entry.State, entry.Incorporation = "responded", "worker_answer"
	entry.DeliveredAt, entry.SettledAt, entry.AppliedAt = &now, &now, &now
	entry.AppliedInstructionVersion = root.InstructionVersion
	return []*Task{parent, next}, nil
}
