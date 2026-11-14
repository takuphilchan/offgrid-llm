package agents

import (
	"math"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

type WorkerIntegrationReservation struct {
	Request   execution.IntegrateChangeSetRequest `json:"request"`
	CallID    string                              `json:"call_id"`
	BaseBytes int64                               `json:"base_bytes"`
	State     string                              `json:"state"`
	Result    *WorkerIntegrationResult            `json:"result,omitempty"`
}

// Full per-file evidence stays in the host journal; context carries a bounded
// reference and actual outcome, without grants or supervisor-private paths.
type WorkerIntegrationResult struct {
	ID           string `json:"id"`
	ChangeSetID  string `json:"change_set_id"`
	State        string `json:"state"`
	Total        int    `json:"total"`
	Integrated   int    `json:"integrated"`
	Error        string `json:"error,omitempty"`
	ConflictPath string `json:"conflict_path,omitempty"`
}

func integrationCallCurrent(root *Task, callID, workerID string) bool {
	if root == nil || root.ParentID != "" || root.WorkID == "" || root.Origin == nil || root.Status != TaskRunning || root.ControlPending || root.ControlAction == "stop" || len(root.PendingInstructions) > 0 || hasConversationHold(root) || root.Checkpoint == nil || root.Config.Execution == nil || root.Config.ExecutionWorkspace == nil {
		return false
	}
	cp := root.Checkpoint
	d, _ := execution.Descriptor(execution.IntegrateTool)
	if cp.ExecutingCall != callID || cp.ExecutingCapability == nil || *cp.ExecutingCapability != d || len(cp.Calls) != 1 || cp.Calls[0].ID != callID || cp.Calls[0].Function.Name != execution.IntegrateTool || !execution.Permitted(execution.IntegrateTool, root.Config.ExecutionWorkspace.Permissions) || root.Config.ExecutionPolicy != nil && !execution.Permitted(execution.IntegrateTool, *root.Config.ExecutionPolicy) {
		return false
	}
	var args struct {
		WorkerRunID string `json:"worker_run_id"`
	}
	return decodeRuntimeArguments(cp.Calls[0].Function.Arguments, &args) == nil && args.WorkerRunID == workerID
}

func (m *Manager) ReserveWorkerIntegration(rootID, actor, workerID, callID string) (req execution.IntegrateChangeSetRequest, fresh bool, resultErr error) {
	_, resultErr = m.updateTask(rootID, func(root *Task) error {
		if root.Actor != actor || !integrationCallCurrent(root, callID, workerID) || root.Checkpoint.StorageBudget == nil {
			return ErrRunConflict
		}
		b := root.Checkpoint.StorageBudget
		if b.Version != 1 || b.ChargedBytes < 0 || b.ChargedBytes > b.LimitBytes {
			return ErrRunConflict
		}
		child := m.tasks[workerID]
		if child == nil || child.Actor != actor || child.ParentID != root.ID || child.Status != TaskCompleted || child.Config.ExecutionWorkerMode != "test" || !workerCaptureSettled(child) {
			return ErrRunConflict
		}
		for key, entry := range b.Reservations {
			if entry.Request.WorkerRunID != workerID || entry.Capture == nil || entry.Capture.Result == nil {
				continue
			}
			if entry.Integration != nil {
				req = entry.Integration.Request
				return nil
			}
			output := entry.Capture.Result
			c := output.Candidate
			if entry.State != "ready" || output.State != "sealed" || output.Validation.State != "passed" || c == nil || c.State != "ready" || output.InstructionVersion != root.InstructionVersion || output.ControlEpoch != root.ControlEpoch || child.InstructionVersion != root.InstructionVersion || entry.Request.Parent.Job != root.ID || entry.ChargedBytes < 0 || entry.Request.MaxBytes < 1 || entry.Request.MaxBytes > math.MaxInt64/2 || !m.integrationCoversEdits(root, entry) {
				return ErrRunConflict
			}
			ref := root.Config.Execution
			expected := execution.Binding{Actor: actor, Workspace: root.Origin.Workspace, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: root.ID}
			if entry.Request.Parent != expected {
				return ErrRunConflict
			}
			extra := min(2*entry.Request.MaxBytes, b.LimitBytes-b.ChargedBytes)
			if extra > math.MaxInt64-entry.ChargedBytes {
				return ErrRunConflict
			}
			req = execution.IntegrateChangeSetRequest{RequestID: "integrate-" + workerID, Source: expected, ChangeSetID: c.ChangeSetID, OperationsDigest: c.OperationsDigest, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch, MaxReservedBytes: entry.ChargedBytes + extra}
			entry.Integration = &WorkerIntegrationReservation{Request: req, CallID: callID, State: "requested", BaseBytes: entry.ChargedBytes}
			entry.ChargedBytes += extra
			b.ChargedBytes += extra
			b.Reservations[key] = entry
			fresh = true
			return nil
		}
		return ErrRunConflict
	})
	if resultErr != nil {
		return execution.IntegrateChangeSetRequest{}, false, resultErr
	}
	return
}

// All completed editing branches must have been tested together. Recursion is
// over bounded, exact captured input references, not model-supplied filenames.
func (m *Manager) integrationCoversEdits(root *Task, test StorageReservation) bool {
	entries := map[string]StorageReservation{}
	for _, entry := range root.Checkpoint.StorageBudget.Reservations {
		if entry.Capture != nil && entry.Capture.Result != nil {
			entries[entry.Capture.Result.ChangeSetID] = entry
		}
	}
	seen := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(execution.ChangeSetReference) bool
	visit = func(ref execution.ChangeSetReference) bool {
		entry, ok := entries[ref.ID]
		if !ok {
			return false
		}
		out := entry.Capture.Result
		source := entry.Request.Parent
		sourceRoot := m.tasks[source.Job]
		source.Job = test.Request.Parent.Job
		if out.State != "sealed" || out.OperationsDigest != ref.OperationsDigest || out.InstructionVersion != root.InstructionVersion || out.InstructionVersion != ref.InstructionVersion || out.ControlEpoch != ref.ControlEpoch || source != test.Request.Parent || ref.SourceJob != entry.Request.Parent.Job || sourceRoot == nil || sourceRoot.Actor != root.Actor || sourceRoot.WorkID != root.WorkID || visiting[ref.ID] {
			return false
		}
		if seen[ref.ID] {
			return true
		}
		visiting[ref.ID] = true
		for _, prior := range entry.Request.Prerequisites {
			if prior.SourceJob == "" {
				prior.SourceJob = entry.Request.Parent.Job
			}
			if !visit(prior) {
				return false
			}
		}
		delete(visiting, ref.ID)
		seen[ref.ID] = true
		return true
	}
	for _, ref := range test.Request.Prerequisites {
		if ref.SourceJob == "" {
			ref.SourceJob = test.Request.Parent.Job
		}
		if !visit(ref) {
			return false
		}
	}
	for _, link := range root.ChildHistory {
		if link.Invalidated {
			continue
		}
		child := m.tasks[link.ID]
		if child == nil || child.ParentID != root.ID || child.Actor != root.Actor {
			return false
		}
		if !workerCaptureTerminal(child) {
			return false
		}
		if child.Config.ExecutionWorkerMode != "edit" || child.Status != TaskCompleted || child.InstructionVersion != root.InstructionVersion {
			continue
		}
		out := workerCapturedOutput(root, child.ID)
		if out == nil || out.State != "sealed" || out.OperationCount > 0 && !seen[out.ChangeSetID] {
			return false
		}
	}
	return len(seen) > 0
}

func (m *Manager) WorkerIntegrationControlCurrent(rootID, actor, workerID, callID string, req execution.IntegrateChangeSetRequest) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	root := m.tasks[rootID]
	if root == nil || root.Actor != actor || !integrationCallCurrent(root, callID, workerID) || root.InstructionVersion != req.InstructionVersion || root.ControlEpoch != req.ControlEpoch || root.Checkpoint.StorageBudget == nil {
		return false
	}
	for _, entry := range root.Checkpoint.StorageBudget.Reservations {
		if entry.Request.WorkerRunID == workerID && entry.Integration != nil && entry.Integration.Request == req && entry.Integration.CallID == callID && (entry.Integration.State == "requested" || entry.Integration.State == "accepted") {
			return true
		}
	}
	return false
}

func (m *Manager) RecordWorkerIntegration(rootID, actor, workerID string, req execution.IntegrateChangeSetRequest, reply execution.WorkerArtifactReply) error {
	_, err := m.updateTask(rootID, func(root *Task) error {
		if root.Actor != actor || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil {
			return ErrRunConflict
		}
		b := root.Checkpoint.StorageBudget
		for key, entry := range b.Reservations {
			i := entry.Integration
			if entry.Request.WorkerRunID != workerID || i == nil || i.Request != req {
				continue
			}
			if entry.Capture == nil || entry.Capture.Result == nil || entry.Capture.Result.Candidate == nil {
				return ErrRunConflict
			}
			if reply.State == "accepted" {
				if i.Result != nil {
					return ErrRunConflict
				}
				i.State = "accepted"
				b.Reservations[key] = entry
				return nil
			}
			result := &WorkerIntegrationResult{ID: execution.IntegrationRequestID(req), ChangeSetID: req.ChangeSetID, State: reply.State, Error: reply.Error}
			switch reply.State {
			case "integrated", "partial", "conflict", "uncertain", "cancelled", "abandoned":
			default:
				return ErrRunConflict
			}
			if page := reply.Integration; page != nil {
				if page.ID != result.ID || page.ChangeSetID != req.ChangeSetID || page.Total < 1 || page.Total > 2*entry.Capture.Result.Candidate.OperationCount || page.Integrated < 0 || page.Integrated > page.Total || page.State != reply.State && reply.State != "uncertain" || reply.State == "integrated" && page.Integrated != page.Total {
					return ErrRunConflict
				}
				result.Total, result.Integrated, result.ConflictPath = page.Total, page.Integrated, page.ConflictPath
				if reply.State == "abandoned" && (page.Integrated != 0 || page.Error != execution.ErrUnsupported.Error()) {
					return ErrRunConflict
				}
				if result.Error == "" {
					result.Error = page.Error
				}
			} else if reply.State != "uncertain" && reply.State != "cancelled" {
				return ErrRunConflict
			}
			if i.Result != nil && i.State != "uncertain" {
				if *i.Result != *result {
					return ErrRunConflict
				}
				return nil
			}
			if record := reply.Copy; record != nil {
				if record.ID != entry.CopyID || record.Source != req.Source || record.WorkerRunID != workerID || record.Purpose != entry.Request.Purpose || record.State != "ready" || !record.UsageMeasured || record.ReservedBytes < 0 || record.MeasuredBytes < 0 || record.MeasuredBytes > record.ReservedBytes {
					return ErrRunConflict
				}
				charge := max(i.BaseBytes, record.ReservedBytes)
				if reply.State == "uncertain" {
					// Unknown outcomes cannot refund capacity, but actual growth
					// must still be charged even when it exceeds the allowance.
					charge = max(charge, entry.ChargedBytes)
				}
				if b.ChargedBytes < entry.ChargedBytes || charge > math.MaxInt64-(b.ChargedBytes-entry.ChargedBytes) {
					return ErrRunConflict
				}
				b.ChargedBytes = b.ChargedBytes - entry.ChargedBytes + charge
				entry.ChargedBytes = charge
				entry.MeasuredBytes = record.MeasuredBytes
				entry.UsageMeasured = true
			}
			i.State, i.Result = reply.State, result
			entry.Capture.Result.Integration = result
			b.Reservations[key] = entry
			return nil
		}
		return ErrRunConflict
	})
	return err
}

func WorkerIntegrationOutput(root *Task, workerID string) *WorkerIntegrationResult {
	if out := workerCapturedOutput(root, workerID); out != nil && out.Integration != nil {
		copy := *out.Integration
		return &copy
	}
	return nil
}
