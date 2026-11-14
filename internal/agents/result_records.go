package agents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// ResultRecord freezes the output and its provenance at settlement. A later
// attempt, archive update or correction cannot replace the bytes behind a
// previously handed-off reference. Reading this internal record is not an
// authorization decision; callers still check its owning run and live sources.
type ResultRecord struct {
	IdentityVersion       int        `json:"identity_version,omitempty"`
	ID                    string     `json:"id"`
	Revision              int64      `json:"revision"`
	WorkID                string     `json:"work_id,omitempty"`
	ConversationID        string     `json:"conversation_id,omitempty"`
	WorkerID              string     `json:"worker_id,omitempty"`
	RunID                 string     `json:"run_id"`
	InstructionVersion    uint64     `json:"instruction_version"`
	ControlEpoch          uint64     `json:"control_epoch"`
	Status                TaskStatus `json:"status"`
	Content               string     `json:"content"`
	Error                 string     `json:"error,omitempty"`
	SourceDependencies    []string   `json:"source_dependencies"`
	EvidenceSteps         []int      `json:"evidence_steps"`
	IndependentlyVerified bool       `json:"independently_verified"`
}

func resultControlScope(task *Task) (ControlScope, bool) {
	if task.Worker != nil {
		return ControlScope{Workspace: task.Worker.Workspace, Actor: task.Actor}, true
	}
	if task.Origin != nil && (task.WorkID != "" || task.ConversationTurn != nil) {
		return ControlScope{Workspace: task.Origin.Workspace, Actor: task.Actor}, true
	}
	return ControlScope{}, false
}

func syncControlResult(tx *sql.Tx, task *Task) error {
	scope, canonical := resultControlScope(task)
	if !canonical || !terminalControlTask(task) || task.Result == "" {
		return nil
	}
	id := resultReference(task)
	previous, err := readControlRecord(tx, scope, id, "result")
	if err == nil {
		var saved ResultRecord
		if json.Unmarshal(previous.Snapshot, &saved) != nil || saved.ID != id || saved.RunID != task.ID || saved.Content != task.Result {
			return ErrRunStorage
		}
		return nil
	}
	if !errors.Is(err, ErrControlNotFound) {
		return err
	}
	result := ResultRecord{IdentityVersion: task.Config.ResultIdentityVersion, ID: id, Revision: 1, WorkID: task.WorkID, RunID: task.ID, InstructionVersion: task.InstructionVersion, ControlEpoch: task.ControlEpoch, Status: task.Status, Content: task.Result, Error: task.Error, SourceDependencies: resultDependencies(task), EvidenceSteps: []int{}}
	if task.Origin != nil {
		result.ConversationID = task.Origin.Conversation
	}
	if task.Worker != nil {
		result.WorkID, result.WorkerID = task.Worker.WorkID, task.Worker.SessionID
	}
	for _, step := range task.Steps {
		if step.Type == "action" {
			result.EvidenceSteps = append(result.EvidenceSteps, step.ID)
		}
	}
	if err = putControlRecord(tx, scope, id, "result", 1, result); err != nil {
		return err
	}
	// The event refers to the record; large output is not copied into the outbox.
	_, err = appendControlEvent(tx, scope, id, 1, "result_sealed", map[string]any{"id": id, "run_id": task.ID, "work_id": result.WorkID}, "")
	return err
}

func resultDependencies(task *Task) []string {
	dependencies := map[string]bool{}
	for _, id := range task.Config.KnowledgeDependencies {
		dependencies[id] = true
	}
	for _, doc := range task.Config.KnowledgeDocuments {
		dependencies[doc.ID] = true
	}
	for _, source := range task.Config.KnowledgeSources {
		dependencies[source.DocumentID] = true
	}
	ids := []string{}
	for id := range dependencies {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (m *Manager) ResultRecord(scope ControlScope, reference string) (*ResultRecord, error) {
	parts := strings.Split(reference, ":")
	if len(parts) != 3 || parts[0] != "result" || !validTaskID(parts[1]) || len(parts[2]) != 64 {
		return nil, ErrControlNotFound
	}
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	record, err := readControlRecord(db, scope, reference, "result")
	if err != nil {
		return nil, err
	}
	var result ResultRecord
	if json.Unmarshal(record.Snapshot, &result) != nil || result.ID != reference || result.RunID != parts[1] || result.IdentityVersion < 0 || result.IdentityVersion > 1 {
		return nil, ErrRunStorage
	}
	probe := &Task{ID: result.RunID, Result: result.Content, Status: result.Status, InstructionVersion: result.InstructionVersion, ControlEpoch: result.ControlEpoch, Config: AgentConfig{ResultIdentityVersion: result.IdentityVersion, KnowledgeDependencies: result.SourceDependencies}}
	if resultReference(probe) != reference {
		return nil, ErrRunStorage
	}
	return &result, nil
}

// Freeze source access independently of changes to a run's mutable config.
func applyResultRecord(task *Task, result *ResultRecord) {
	task.Result, task.Error, task.Status = result.Content, result.Error, result.Status
	task.InstructionVersion, task.ControlEpoch = result.InstructionVersion, result.ControlEpoch
	task.Config.KnowledgeDependencies = append(task.Config.KnowledgeDependencies, result.SourceDependencies...)
}

// Backfill only recorded terminal outputs. This does not resume execution or
// manufacture verification, grants, missing output or new instruction history.
func migrateControlResults(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT snapshot FROM agent_tasks ORDER BY id`)
	if err != nil {
		return err
	}
	var tasks []*Task
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var task Task
		if err = json.Unmarshal(raw, &task); err != nil {
			rows.Close()
			return err
		}
		if _, canonical := resultControlScope(&task); canonical {
			tasks = append(tasks, &task)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err = syncControlResult(tx, task); err != nil {
			return err
		}
		if err = syncControlChanges(tx, task); err != nil {
			return err
		}
	}
	return nil
}
