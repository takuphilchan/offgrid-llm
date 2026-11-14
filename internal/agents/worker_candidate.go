package agents

import (
	"slices"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// WorkerPrerequisiteChanges resolves only recorded assignment dependencies and
// their exact captured outputs. The model cannot nominate native artifact IDs.
func WorkerPrerequisiteChanges(root *Task, childID string) ([]execution.ChangeSetReference, error) {
	if root == nil {
		return nil, ErrRunConflict
	}
	if root.Delegation == nil {
		return nil, nil
	}
	var assignment *ChildLink
	for i := range root.Delegation.Children {
		if root.Delegation.Children[i].ID == childID {
			assignment = &root.Delegation.Children[i]
			break
		}
	}
	if assignment == nil || assignment.Invalidated {
		return nil, ErrRunConflict
	}
	refs := []execution.ChangeSetReference{}
	for _, key := range assignment.Spec.DependsOn {
		var dependency *ChildLink
		for i := range root.Delegation.Children {
			if root.Delegation.Children[i].Spec.Key == key {
				dependency = &root.Delegation.Children[i]
				break
			}
		}
		if dependency == nil || dependency.Invalidated {
			return nil, ErrRunConflict
		}
		if !executionWorkerMode(dependency.Spec.Mode) {
			continue
		}
		output := workerCapturedOutput(root, dependency.ID)
		if output == nil || output.State != "sealed" || output.InstructionVersion != root.InstructionVersion || output.ControlEpoch != root.ControlEpoch {
			return nil, ErrRunConflict
		}
		refs = append(refs, execution.ChangeSetReference{ID: output.ChangeSetID, OperationsDigest: output.OperationsDigest, InstructionVersion: output.InstructionVersion, ControlEpoch: output.ControlEpoch})
	}
	return refs, nil
}

// Caller holds the manager lock. Check live dependency outcomes as well as the
// stored references at reservation and again at final environment binding.
func (m *Manager) workerCandidateMatches(root *Task, req execution.WorkingCopyRequest) bool {
	assignment := m.tasks[req.WorkerRunID]
	refs, err := WorkerInputChanges(root, assignment)
	if err != nil || !slices.Equal(refs, req.Prerequisites) || !slices.Equal(WorkerIntegratedInputs(root), req.Integrated) {
		return false
	}
	for _, ref := range refs {
		valid := false
		for _, reservation := range root.Checkpoint.StorageBudget.Reservations {
			if reservation.Capture == nil || reservation.Capture.Result == nil || reservation.Capture.Result.ChangeSetID != ref.ID {
				continue
			}
			child := m.tasks[reservation.Request.WorkerRunID]
			valid = child != nil && child.Actor == root.Actor && child.Worker != nil && child.Worker.WorkID == root.WorkID && child.ParentID == reservation.Request.Parent.Job && child.InstructionVersion == root.InstructionVersion
			if ref.SourceJob != "" {
				retained := assignment.Checkpoint.WorkerContinuation
				valid = valid && workerCaptureSettled(child) && retained != nil && retained.PreviousRunID == child.ID && retained.Input == ref && child.Worker.SessionID == assignment.Worker.SessionID
			} else {
				valid = valid && child.Status == TaskCompleted
			}
			break
		}
		if !valid {
			return false
		}
	}
	return true
}
