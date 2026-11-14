package agents

import "github.com/takuphilchan/offgrid-llm/internal/execution"

type WorkerEnvironment struct {
	ParentRunID string `json:"parent_run_id"`
	RequestID   string `json:"request_id"`
	CopyID      string `json:"copy_id"`
}

// BindWorkerEnvironment installs only a service-registered native resource after
// its storage receipt. A concurrent Stop/correction wins this final boundary.
func (m *Manager) BindWorkerEnvironment(id, actor string, req execution.WorkingCopyRequest, resource execution.Resource) error {
	_, err := m.updateTask(req.WorkerRunID, func(child *Task) error {
		root := m.tasks[id]
		if root == nil || root.Actor != actor || child.Actor != actor || child.ParentID != id || child.Worker == nil || child.Worker.WorkID != root.WorkID || child.Status != TaskPending || child.Checkpoint == nil || root.Checkpoint == nil || root.Checkpoint.StorageBudget == nil || root.Status != TaskChildren || root.ControlPending || child.ControlPending || root.ControlAction == "stop" || child.ControlAction == "stop" || hasConversationHold(root) || hasConversationHold(child) || len(root.PendingInstructions) > 0 || len(child.PendingInstructions) > 0 || root.InstructionVersion != req.InstructionVersion || root.ControlEpoch != req.ControlEpoch || child.InstructionVersion != root.InstructionVersion {
			return ErrRunConflict
		}
		if !m.workerCandidateMatches(root, req) {
			return ErrRunConflict
		}
		reserved, exists := root.Checkpoint.StorageBudget.Reservations[storageReservationKey(req)]
		if !exists || reserved.Digest != storageRequestDigest(req) || reserved.State != "ready" || reserved.CopyID == "" {
			return ErrRunConflict
		}
		if resource.Reference.Validate() != nil || resource.Reference.Host != req.Parent.Host || resource.Reference.Session == req.Parent.Session || resource.Reference.Grant == req.Parent.Grant || resource.Backend != "local" || resource.Root == "" || resource.Permissions != req.Permissions || child.Config.ExecutionPolicy != nil && !execution.PermissionsWithin(resource.Permissions, *child.Config.ExecutionPolicy) {
			return ErrRunConflict
		}
		environment := WorkerEnvironment{ParentRunID: id, RequestID: req.RequestID, CopyID: reserved.CopyID}
		if previous := child.Checkpoint.WorkerEnvironment; previous != nil {
			if *previous != environment || child.Config.Execution == nil || *child.Config.Execution != resource.Reference {
				return ErrRunConflict
			}
			return nil
		}
		if child.Config.Execution != nil {
			return ErrRunConflict
		}
		child.Config.Execution = &resource.Reference
		child.Config.ExecutionWorkspace = &resource
		child.Checkpoint.WorkerEnvironment = &environment
		return nil
	})
	return err
}
