package agents

import (
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"sort"
)

// Completed effects stay with the shared ledger across work attempts. These
// references never restore an execution grant; the host verifies each journal.
func WorkerIntegratedInputs(root *Task) []execution.IntegrationReference {
	if root == nil || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil || root.Config.Execution == nil || root.Origin == nil {
		return nil
	}
	ref := root.Config.Execution
	expected := execution.Binding{Actor: root.Actor, Workspace: root.Origin.Workspace, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: root.ID}
	refs := []execution.IntegrationReference{}
	seen := map[string]bool{}
	for _, entry := range root.Checkpoint.StorageBudget.Reservations {
		i := entry.Integration
		if i == nil || i.State != "integrated" || i.Result == nil || i.Result.State != "integrated" || i.Result.Total < 1 || i.Result.Total != i.Result.Integrated || i.Request.InstructionVersion != root.InstructionVersion || i.Request.ControlEpoch > root.ControlEpoch || i.Result.ID != execution.IntegrationRequestID(i.Request) {
			continue
		}
		source := i.Request.Source
		source.Job = root.ID
		if source != expected || seen[i.Result.ID] {
			continue
		}
		seen[i.Result.ID] = true
		refs = append(refs, execution.IntegrationReference{ID: i.Result.ID, SourceJob: i.Request.Source.Job})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	return refs
}

// A follow-up pins the previous settled assignment's immutable file result.
// This carries content provenance, never a child execution session or grant.
type WorkerContinuation struct {
	PreviousRunID string                       `json:"previous_run_id"`
	Input         execution.ChangeSetReference `json:"input"`
}

func retainedWorkerInput(parent, previous *Task) (*WorkerContinuation, error) {
	if !workerCaptureSettled(previous) || parent.Checkpoint == nil || parent.Checkpoint.StorageBudget == nil {
		return nil, ErrRunConflict
	}
	for _, reservation := range parent.Checkpoint.StorageBudget.Reservations {
		if reservation.Request.WorkerRunID != previous.ID || reservation.Capture == nil || reservation.Capture.Result == nil {
			continue
		}
		result := reservation.Capture.Result
		if result.State != "sealed" || reservation.Capture.State != "sealed" || result.InstructionVersion != parent.InstructionVersion || reservation.Request.Parent.Job != previous.ParentID {
			return nil, ErrRunConflict
		}
		return &WorkerContinuation{PreviousRunID: previous.ID, Input: execution.ChangeSetReference{SourceJob: reservation.Request.Parent.Job, ID: result.ChangeSetID, OperationsDigest: result.OperationsDigest, InstructionVersion: result.InstructionVersion, ControlEpoch: result.ControlEpoch}}, nil
	}
	return nil, ErrRunConflict
}

func WorkerInputChanges(root, child *Task) ([]execution.ChangeSetReference, error) {
	if root == nil || child == nil || child.ParentID != root.ID {
		return nil, ErrRunConflict
	}
	refs, err := WorkerPrerequisiteChanges(root, child.ID)
	if err != nil {
		return nil, err
	}
	if child.Checkpoint != nil && child.Checkpoint.WorkerContinuation != nil {
		input := child.Checkpoint.WorkerContinuation.Input
		if input.InstructionVersion != root.InstructionVersion || input.ControlEpoch > root.ControlEpoch {
			return nil, ErrRunConflict
		}
		refs = append([]execution.ChangeSetReference{input}, refs...)
	}
	return refs, nil
}

// BindWorkerSource is called only after the service resolves the selected host
// against its live registry. Saved work never restores authority by itself.
func (m *Manager) BindWorkerSource(id, actor string, resource execution.Resource) (*Task, error) {
	return m.updateTask(id, func(root *Task) error {
		if root.Actor != actor || root.ParentID != "" || root.WorkID == "" || root.Checkpoint == nil || root.Status != TaskChildren || root.ControlPending || root.ControlAction == "stop" || hasConversationHold(root) || len(root.PendingInstructions) > 0 {
			return ErrRunConflict
		}
		selected := root.Checkpoint.WorkerSource
		if selected == nil || selected.Reference != resource.Reference || selected.Root != resource.Root || selected.Backend != resource.Backend || !resource.PreparedWorkers || resource.Backend != "local" || !execution.PermissionsWithin(resource.Permissions, selected.Permissions) {
			return ErrRunConflict
		}
		if root.Config.Execution != nil && *root.Config.Execution != resource.Reference {
			return ErrRunConflict
		}
		root.Config.Execution, root.Config.ExecutionWorkspace = &resource.Reference, &resource
		return nil
	})
}
