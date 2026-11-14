package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// A capture allocation is a subreservation of the same root ledger. Only an
// exact sealed result plus measured native storage can settle unused capacity.
type WorkerCaptureReservation struct {
	Request   execution.SealChangeSetRequest `json:"request"`
	BaseBytes int64                          `json:"base_bytes"`
	State     string                         `json:"state"`
	Result    *WorkerCapturedOutput          `json:"result,omitempty"`
}

// Safe projection for parent context and client inspection: no host grant or
// provider-local paths. A sealed proposal has not been validated or integrated.
type WorkerCapturedOutput struct {
	Integration        *WorkerIntegrationResult    `json:"integration,omitempty"`
	Candidate          *WorkerIntegrationCandidate `json:"candidate,omitempty"`
	Validation         execution.ValidationSummary `json:"validation,omitzero"`
	State              string                      `json:"state"`
	ChangeSetID        string                      `json:"change_set_id,omitempty"`
	OperationsDigest   string                      `json:"operations_digest,omitempty"`
	OperationCount     int                         `json:"operation_count,omitempty"`
	InstructionVersion uint64                      `json:"instruction_version"`
	ControlEpoch       uint64                      `json:"control_epoch"`
}

func workerCapturedOutput(parent *Task, childID string) *WorkerCapturedOutput {
	if parent.Checkpoint != nil && parent.Checkpoint.StorageBudget != nil {
		for _, reservation := range parent.Checkpoint.StorageBudget.Reservations {
			if reservation.Request.WorkerRunID == childID && reservation.Capture != nil {
				return reservation.Capture.Result
			}
		}
	}
	return nil
}

func workerCapturePending(parent, child *Task) bool {
	return workerCaptureTerminal(child) && executionWorkerMode(child.Config.ExecutionWorkerMode) && HasWorkerEnvironment(child) && workerCapturedOutput(parent, child.ID) == nil
}

func workerCaptureTerminal(child *Task) bool {
	return child != nil && (child.Status == TaskCompleted || child.Status == TaskFailed || child.Status == TaskCancelled)
}

func workerCaptureSettled(child *Task) bool {
	return workerCaptureTerminal(child) && !child.ControlPending && !child.ProcessStopPending && child.Checkpoint != nil && child.Checkpoint.ExecutingCall == ""
}

func workerDependencyUnavailable(parent, child *Task) bool {
	if child == nil {
		return false
	}
	output := workerCapturedOutput(parent, child.ID)
	return output != nil && output.State != "sealed"
}

func workerDependencyComplete(parent, child *Task) bool {
	return child != nil && child.Status == TaskCompleted && !workerCapturePending(parent, child) && !workerDependencyUnavailable(parent, child)
}

// ReserveWorkerCapture returns fresh only to the controller which committed the
// first intent. Other calls must inspect the native receipt, never begin again.
func (m *Manager) ReserveWorkerCapture(rootID, actor, childID string) (request execution.SealChangeSetRequest, fresh bool, resultErr error) {
	_, resultErr = m.updateTask(rootID, func(root *Task) error {
		child := m.tasks[childID]
		if root.Actor != actor || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil || child == nil || child.Actor != actor || child.ParentID != root.ID || !HasWorkerEnvironment(child) || !workerCaptureSettled(child) || root.Status != TaskChildren || root.ControlPending || root.ControlAction == "stop" || hasConversationHold(root) || hasConversationHold(child) || len(root.PendingInstructions) > 0 {
			return ErrRunConflict
		}
		budget := root.Checkpoint.StorageBudget
		if budget.Version != 1 || budget.ChargedBytes < 0 || budget.LimitBytes < 1 {
			return ErrRunConflict
		}
		for key, reservation := range budget.Reservations {
			if reservation.Request.WorkerRunID != child.ID || reservation.CopyID != child.Checkpoint.WorkerEnvironment.CopyID {
				continue
			}
			if reservation.Capture != nil {
				request = reservation.Capture.Request
				return nil
			}
			if reservation.State != "ready" || reservation.ChargedBytes < 0 || reservation.Request.MaxBytes <= 0 || budget.ChargedBytes < reservation.ChargedBytes || budget.ChargedBytes > budget.LimitBytes {
				return ErrRunConflict
			}
			extra := min(reservation.Request.MaxBytes, budget.LimitBytes-budget.ChargedBytes)
			if extra > math.MaxInt64-reservation.ChargedBytes {
				return ErrRunConflict
			}
			request = execution.SealChangeSetRequest{RequestID: "capture-" + child.ID, CopyID: reservation.CopyID, Source: reservation.Request.Parent, InstructionVersion: reservation.Request.InstructionVersion, ControlEpoch: reservation.Request.ControlEpoch, MaxReservedBytes: reservation.ChargedBytes + extra}
			reservation.Capture = &WorkerCaptureReservation{Request: request, BaseBytes: reservation.ChargedBytes, State: "requested"}
			reservation.ChargedBytes += extra
			budget.ChargedBytes += extra
			budget.Reservations[key] = reservation
			fresh = true
			return nil
		}
		return ErrRunConflict
	})
	if resultErr != nil {
		return execution.SealChangeSetRequest{}, false, resultErr
	}
	return
}

func (m *Manager) RecordWorkerCapture(rootID, actor, childID string, req execution.SealChangeSetRequest, reply execution.WorkerArtifactReply) error {
	_, err := m.updateTask(rootID, func(root *Task) error {
		child := m.tasks[childID]
		if root.Actor != actor || child == nil || child.Actor != actor || child.ParentID != root.ID || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil {
			return ErrRunConflict
		}
		budget := root.Checkpoint.StorageBudget
		if budget.Version != 1 || budget.ChargedBytes < 0 {
			return ErrRunConflict
		}
		for key, reservation := range budget.Reservations {
			capture := reservation.Capture
			if reservation.Request.WorkerRunID != childID || capture == nil || capture.Request != req {
				continue
			}
			if capture.State == "sealed" {
				if reply.State != "sealed" || reply.ChangeSet == nil || capture.Result == nil || capture.Result.ChangeSetID != reply.ChangeSet.ID || capture.Result.OperationsDigest != reply.ChangeSet.OperationsDigest || capture.Result.Validation != reply.ChangeSet.Validation {
					return ErrRunConflict
				}
				candidate, err := capturedIntegrationCandidate(req, childID, reply)
				if err != nil || !sameIntegrationCandidate(capture.Result.Candidate, candidate) {
					return ErrRunConflict
				}
				return nil
			}
			if reply.State == "accepted" {
				if capture.Result != nil {
					return ErrRunConflict
				}
				capture.State = "accepted"
				budget.Reservations[key] = reservation
				return nil
			}
			output := &WorkerCapturedOutput{State: reply.State, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch}
			switch reply.State {
			case "sealed":
				set, record := reply.ChangeSet, reply.Copy
				identity, _ := json.Marshal(struct {
					Source            execution.Binding
					CopyID, RequestID string
				}{req.Source, req.CopyID, req.RequestID})
				hash := sha256.Sum256(identity)
				if set == nil || record == nil || set.Version != 1 || set.ID != hex.EncodeToString(hash[:]) || set.State != "sealed" || set.Source != req.Source || set.WorkerRunID != childID || set.CopyID != req.CopyID || set.InstructionVersion != req.InstructionVersion || set.ControlEpoch != req.ControlEpoch || len(set.OperationsDigest) != 64 || set.OperationCount < 0 || record.ID != req.CopyID || record.Source != req.Source || record.WorkerRunID != childID || record.State != "ready" || !record.UsageMeasured || record.ReservedBytes < 0 || record.MeasuredBytes < 0 || record.ReservedBytes > req.MaxReservedBytes || record.MeasuredBytes > record.ReservedBytes {
					return ErrRunConflict
				}
				if _, err := hex.DecodeString(set.OperationsDigest); err != nil {
					return ErrRunConflict
				}
				if !set.Validation.ValidFor(*record) {
					return ErrRunConflict
				}
				candidate, err := capturedIntegrationCandidate(req, childID, reply)
				if err != nil {
					return err
				}
				output.Candidate = candidate
				charge := max(capture.BaseBytes, record.ReservedBytes)
				if charge > reservation.ChargedBytes || budget.ChargedBytes < reservation.ChargedBytes {
					return ErrRunConflict
				}
				budget.ChargedBytes -= reservation.ChargedBytes - charge
				reservation.ChargedBytes = charge
				reservation.MeasuredBytes, reservation.UsageMeasured = record.MeasuredBytes, true
				output.ChangeSetID, output.OperationsDigest, output.OperationCount = set.ID, set.OperationsDigest, set.OperationCount
				output.Validation = set.Validation
			case "uncertain": // Unknown outcomes keep the complete reservation.
			default:
				return ErrRunConflict
			}
			capture.State, capture.Result = reply.State, output
			budget.Reservations[key] = reservation
			return nil
		}
		return ErrRunConflict
	})
	return err
}
