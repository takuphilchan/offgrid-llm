package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// StorageBudget belongs to the root work, independently of per-run model
// allocations. Closing workers or restarting the service never refunds it.
type StorageBudget struct {
	Version      int                           `json:"version"`
	LimitBytes   int64                         `json:"limit_bytes"`
	ChargedBytes int64                         `json:"charged_bytes"`
	Reservations map[string]StorageReservation `json:"reservations"`
}

type StorageReservation struct {
	Integration   *WorkerIntegrationReservation `json:"integration,omitempty"`
	Capture       *WorkerCaptureReservation     `json:"capture,omitempty"`
	Request       execution.WorkingCopyRequest  `json:"request"`
	Digest        string                        `json:"digest"`
	CopyID        string                        `json:"copy_id,omitempty"`
	State         string                        `json:"state"`
	ChargedBytes  int64                         `json:"charged_bytes"`
	MeasuredBytes int64                         `json:"measured_bytes"`
	UsageMeasured bool                          `json:"usage_measured"`
}

func storageReservationKey(req execution.WorkingCopyRequest) string {
	data, _ := json.Marshal(struct {
		Parent execution.Binding
		ID     string
	}{req.Parent, req.RequestID})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func storageRequestDigest(req execution.WorkingCopyRequest) string {
	data, _ := json.Marshal(req)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ReserveWorkerStorage is a trusted controller boundary. The root reservation
// commits before any native preparation; the caller cannot infer authority from
// its success. Host approval and exact child admission are still required.
func (m *Manager) ReserveWorkerStorage(id, actor string, req execution.WorkingCopyRequest, limit int64) error {
	req.RequiredPaths = append([]string(nil), req.RequiredPaths...)
	req.Prerequisites = append([]execution.ChangeSetReference(nil), req.Prerequisites...)
	req.Integrated = append([]execution.IntegrationReference(nil), req.Integrated...)
	if limit <= 0 || limit > math.MaxInt64/128 || req.MaxBytes <= 0 || req.MaxBytes > limit || req.RequestID == "" || len(req.RequestID) > 256 || strings.ContainsAny(req.RequestID, "\x00\r\n") {
		return ErrRunConflict
	}
	_, err := m.updateTask(id, func(root *Task) error {
		if root.Actor != actor || root.ParentID != "" || root.WorkID == "" || root.Origin == nil || root.Checkpoint == nil || root.Config.Execution == nil || root.Config.ExecutionWorkspace == nil {
			return ErrRunConflict
		}
		ref := root.Config.Execution
		expected := execution.Binding{Actor: actor, Workspace: root.Origin.Workspace, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: root.ID}
		if req.Parent != expected || !execution.PermissionsWithin(req.Permissions, root.Config.ExecutionWorkspace.Permissions) || root.Config.ExecutionPolicy != nil && !execution.PermissionsWithin(req.Permissions, *root.Config.ExecutionPolicy) {
			return ErrRunConflict
		}
		budget := root.Checkpoint.StorageBudget
		if budget == nil {
			budget = &StorageBudget{Version: 1, LimitBytes: limit, Reservations: map[string]StorageReservation{}}
			root.Checkpoint.StorageBudget = budget
		}
		if budget.Version != 1 || budget.LimitBytes != limit || budget.ChargedBytes < 0 {
			return ErrRunConflict
		}
		key, digest := storageReservationKey(req), storageRequestDigest(req)
		child := m.tasks[req.WorkerRunID]
		if child == nil || child.Actor != actor || child.ParentID != root.ID || child.Worker == nil || child.Worker.WorkID != root.WorkID || child.Worker.Workspace != root.Origin.Workspace || child.DeletedAt != nil || child.Status != TaskPending || child.ControlPending || child.ControlAction == "stop" || hasConversationHold(child) || hasConversationHold(root) || root.ControlPending || root.ControlAction == "stop" || len(root.PendingInstructions) > 0 || root.Status != TaskChildren || req.InstructionVersion != root.InstructionVersion || req.ControlEpoch != root.ControlEpoch || child.InstructionVersion != root.InstructionVersion {
			return ErrRunConflict
		}
		if !m.workerCandidateMatches(root, req) {
			return ErrRunConflict
		}
		if req.Purpose != "" && req.Purpose != child.Config.ExecutionWorkerMode {
			return ErrRunConflict
		}
		if old, exists := budget.Reservations[key]; exists {
			if old.Digest != digest {
				return ErrRunConflict
			}
			return nil
		}
		linked := false
		for _, link := range root.ChildHistory {
			if link.ID == child.ID && !link.Invalidated {
				linked = true
			}
		}
		if !linked || len(budget.Reservations) >= 128 || req.MaxBytes > budget.LimitBytes-budget.ChargedBytes {
			return ErrRunConflict
		}
		for _, previous := range budget.Reservations {
			if previous.Request.WorkerRunID == req.WorkerRunID {
				return ErrRunConflict
			}
		}
		budget.Reservations[key] = StorageReservation{Request: req, Digest: digest, State: "reserved", ChargedBytes: req.MaxBytes}
		budget.ChargedBytes += req.MaxBytes
		return nil
	})
	return err
}

// RecordWorkerStorage consumes an authenticated host record. Unknown, failed
// and merely closed workers retain capacity. Only a matching released record
// confirms cleanup; shrinkage of a live copy never reopens its reserved space.
func (m *Manager) RecordWorkerStorage(id, actor string, req execution.WorkingCopyRequest, record execution.WorkingCopyRecord) error {
	_, err := m.updateTask(id, func(root *Task) error {
		if root.Actor != actor || root.ParentID != "" || root.WorkID == "" || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil {
			return ErrRunConflict
		}
		b := root.Checkpoint.StorageBudget
		key := storageReservationKey(req)
		old, exists := b.Reservations[key]
		if b.Version != 1 || !exists || old.Digest != storageRequestDigest(req) || record.Source != req.Parent || record.WorkerRunID != req.WorkerRunID || record.Purpose != req.Purpose || record.ID != key || old.CopyID != "" && old.CopyID != record.ID || record.ReservedBytes < 0 || record.MeasuredBytes < 0 {
			return ErrRunConflict
		}
		if _, err := hex.DecodeString(record.ID); err != nil {
			return ErrRunConflict
		}
		if old.State == "released" {
			if record.State != "released" || record.ReservedBytes != 0 {
				return ErrRunConflict
			}
			return nil
		}
		charge := max(old.ChargedBytes, record.ReservedBytes, record.MeasuredBytes)
		switch record.State {
		case "preparing", "ready", "uncertain", "releasing":
		case "released":
			if old.CopyID == "" || record.ReservedBytes != 0 {
				return ErrRunConflict
			}
			charge = 0
		default:
			return ErrRunConflict
		}
		if b.ChargedBytes < old.ChargedBytes || charge > math.MaxInt64-(b.ChargedBytes-old.ChargedBytes) {
			return ErrRunConflict
		}
		b.ChargedBytes = b.ChargedBytes - old.ChargedBytes + charge
		old.CopyID, old.State, old.ChargedBytes = record.ID, record.State, charge
		old.UsageMeasured = record.UsageMeasured
		if record.UsageMeasured {
			old.MeasuredBytes, old.UsageMeasured = record.MeasuredBytes, true
		}
		b.Reservations[key] = old
		return nil
	})
	return err
}
