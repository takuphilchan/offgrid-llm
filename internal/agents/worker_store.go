package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type WorkerBinding struct {
	Workspace string `json:"workspace"`
	WorkID    string `json:"work_id"`
	SessionID string `json:"session_id"`
}

func (m *Manager) WorkerSession(scope ControlScope, id string) (*api.WorkerSession, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	record, err := readControlRecord(db, scope, id, "worker")
	if err != nil {
		return nil, err
	}
	var worker api.WorkerSession
	err = json.Unmarshal(record.Snapshot, &worker)
	return &worker, err
}

// Child identities and their durable session commit in the existing run tx.
// A restart cannot discover a child with no worker or manufacture a new worker.
func syncControlWorker(tx *sql.Tx, task *Task) error {
	if task.Worker == nil {
		return nil
	}
	if task.ParentID == "" || task.WorkID != "" || !validTaskID(task.Worker.SessionID) {
		return ErrRunStorage
	}
	scope := ControlScope{Workspace: task.Worker.Workspace, Actor: task.Actor}
	workRecord, err := readControlRecord(tx, scope, task.Worker.WorkID, "work")
	if err != nil {
		return err
	}
	var work WorkItem
	if json.Unmarshal(workRecord.Snapshot, &work) != nil {
		return ErrRunStorage
	}
	parentKnown := work.RunID == task.ParentID
	for _, id := range work.RunIDs {
		parentKnown = parentKnown || id == task.ParentID && terminalControlTask(task)
	}
	if !parentKnown {
		return ErrRunStorage
	}
	var parentData []byte
	if err = tx.QueryRow(`SELECT snapshot FROM agent_tasks WHERE id=? AND actor=?`, task.ParentID, task.Actor).Scan(&parentData); err != nil {
		return err
	}
	var parent Task
	if json.Unmarshal(parentData, &parent) != nil {
		return ErrRunStorage
	}
	var spec *ChildSpec
	for _, link := range parent.ChildHistory {
		if link.ID == task.ID {
			selected := link.Spec
			spec = &selected
			break
		}
	}
	if spec == nil {
		return ErrRunStorage
	}
	record, err := readControlRecord(tx, scope, task.Worker.SessionID, "worker")
	worker := api.WorkerSession{ID: task.Worker.SessionID, Revision: 1, WorkID: work.ID, RootRunID: task.ParentID, CurrentRunID: task.ID, RunIDs: []string{task.ID}, Assignment: workerAssignment(*spec, task)}
	created := errors.Is(err, ErrControlNotFound)
	if err != nil && !created {
		return err
	}
	if !created {
		if json.Unmarshal(record.Snapshot, &worker) != nil || worker.WorkID != work.ID {
			return ErrRunConflict
		}
		if worker.CurrentRunID != task.ID && terminalControlTask(task) {
			for _, id := range worker.RunIDs {
				if id == task.ID {
					return nil
				}
			}
		}
		if worker.RootRunID != task.ParentID || worker.CurrentRunID != task.ID {
			return ErrRunConflict
		}
	}
	before, _ := json.Marshal(worker)
	worker.Assignment.ModelCalls = task.Config.MaxIterations
	worker.Question = task.WorkerQuestion
	worker.Status, worker.AssignmentVersion = task.Status, task.AssignmentVersion
	if hasConversationHold(task) && task.ControlAction == "" && !terminalControlTask(task) {
		worker.Status = TaskConversationInput
	}
	if task.ControlPending && task.ControlAction == "stop" {
		worker.Status = "stopping"
	}
	if task.ControlPending && task.ControlAction == "pause" {
		worker.Status = "pausing"
	}
	worker.Stopped = worker.Stopped || task.ControlAction == "stop"
	if task.Result != "" && terminalControlTask(task) {
		worker.ResultReference = resultReference(task)
	}
	messagesChanged := false
	for _, instruction := range task.Instructions {
		if !task.WorkerMessageRequests[instruction.RequestID] {
			continue
		}
		message, err := readWorkerMessage(tx, scope, worker.ID, instruction.RequestID)
		if err != nil {
			return err
		}
		if message.State == "applied" {
			continue
		}
		if message.State != "delivered" {
			return ErrRunConflict
		}
		now := time.Now().UTC()
		message.State, message.AppliedAt = "applied", &now
		if err = putWorkerMessage(tx, scope, message); err != nil {
			return err
		}
		messagesChanged = true
	}
	abandoned, err := abandonEndedWorkerMessages(tx, scope, task)
	if err != nil {
		return err
	}
	messagesChanged = messagesChanged || abandoned
	after, _ := json.Marshal(worker)
	if !created && string(before) == string(after) && !messagesChanged {
		return nil
	}
	if !created {
		worker.Revision++
	}
	if err = putControlRecord(tx, scope, worker.ID, "worker", worker.Revision, &worker); err != nil {
		return err
	}
	_, err = appendControlEvent(tx, scope, worker.ID, worker.Revision, "worker_state", &worker, "")
	return err
}

func readWorkerMessage(q controlReader, scope ControlScope, worker, id string) (*api.WorkerMessage, error) {
	var data []byte
	err := q.QueryRow(`SELECT payload FROM agent_worker_messages WHERE workspace=? AND actor=? AND worker_id=? AND message_id=?`, scope.Workspace, scope.Actor, worker, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrControlNotFound
	}
	if err != nil {
		return nil, err
	}
	var message api.WorkerMessage
	err = json.Unmarshal(data, &message)
	return &message, err
}

func putWorkerMessage(tx *sql.Tx, scope ControlScope, message *api.WorkerMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE agent_worker_messages SET payload=? WHERE workspace=? AND actor=? AND worker_id=? AND message_id=?`, data, scope.Workspace, scope.Actor, message.WorkerID, message.ID)
	return err
}
