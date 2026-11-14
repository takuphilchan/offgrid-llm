package agents

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Exact file operations and their blobs remain in the host's journal. The
// control store owns their stable identity, lifecycle and pinned digest. Live
// authority is deliberately excluded, including from historical events.
type ChangeSetRecord struct {
	api.WorkChange
	Revision         int64                       `json:"revision"`
	OperationsDigest string                      `json:"operations_digest"`
	Validation       execution.ValidationSummary `json:"validation"`
	Source           execution.Binding           `json:"-"`
}

func TaskChangeRecords(root *Task) []ChangeSetRecord {
	records := []ChangeSetRecord{}
	if root.WorkID == "" || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil {
		return records
	}
	for _, entry := range root.Checkpoint.StorageBudget.Reservations {
		if entry.Capture == nil || entry.Capture.Result == nil || entry.Capture.Result.ChangeSetID == "" {
			continue
		}
		out := entry.Capture.Result
		base := api.WorkChange{ID: out.ChangeSetID, WorkID: root.WorkID, RunID: entry.Request.Parent.Job, WorkerRunID: entry.Request.WorkerRunID, Kind: "proposal", State: out.State, InstructionVersion: out.InstructionVersion, ControlEpoch: out.ControlEpoch, OperationCount: out.OperationCount, ValidationState: out.Validation.State, ValidationCount: out.Validation.CheckCount}
		records = append(records, ChangeSetRecord{WorkChange: base, Revision: 1, Source: entry.Request.Parent, OperationsDigest: out.OperationsDigest, Validation: out.Validation})
		if candidate := out.Candidate; candidate != nil {
			base.ID, base.Kind, base.State, base.OperationCount = candidate.ChangeSetID, "candidate", candidate.State, candidate.OperationCount
			if integration := entry.Integration; integration != nil {
				base.IntegrationID, base.State = execution.IntegrationRequestID(integration.Request), integration.State
				if integration.State == "requested" || integration.State == "accepted" {
					base.State = "applying"
				}
				if integration.Result != nil {
					base.IntegratedCount = integration.Result.Integrated
				}
			}
			records = append(records, ChangeSetRecord{WorkChange: base, Revision: 1, Source: entry.Request.Parent, OperationsDigest: candidate.OperationsDigest, Validation: out.Validation})
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func syncControlChanges(tx *sql.Tx, task *Task) error {
	if task.WorkID == "" || task.Origin == nil {
		return nil
	}
	scope := ControlScope{Workspace: task.Origin.Workspace, Actor: task.Actor}
	for _, next := range TaskChangeRecords(task) {
		record, err := readControlRecord(tx, scope, next.ID, "change_set")
		if err == nil {
			var previous ChangeSetRecord
			if json.Unmarshal(record.Snapshot, &previous) != nil || previous.RunID != next.RunID || previous.WorkID != next.WorkID || previous.WorkerRunID != next.WorkerRunID || previous.OperationsDigest != next.OperationsDigest || previous.InstructionVersion != next.InstructionVersion || previous.ControlEpoch != next.ControlEpoch {
				return ErrRunConflict
			}
			next.Revision = previous.Revision
			data, _ := json.Marshal(next)
			if bytes.Equal(data, record.Snapshot) {
				continue
			}
			next.Revision++
		} else if !errors.Is(err, ErrControlNotFound) {
			return err
		}
		if err = putControlRecord(tx, scope, next.ID, "change_set", next.Revision, next); err != nil {
			return err
		}
		if _, err = appendControlEvent(tx, scope, next.ID, next.Revision, "change_set_state", next, ""); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) ChangeSetRecord(scope ControlScope, id string) (*ChangeSetRecord, error) {
	db, err := m.controlDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	record, err := readControlRecord(db, scope, id, "change_set")
	if err != nil {
		return nil, err
	}
	var result ChangeSetRecord
	if json.Unmarshal(record.Snapshot, &result) != nil || result.ID != id || result.Revision != record.Revision {
		return nil, ErrRunStorage
	}
	return &result, nil
}
