package server

import (
	"context"
	"fmt"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// The coordinator invokes this outside inference and manager locks. Repeated
// ticks use the same reservation and native request identity, never a new copy.
func (b *browserRunTools) PrepareExecutionAssignment(ctx context.Context, root, child *agents.Task, spec agents.ChildSpec) (bool, error) {
	if root == nil || child == nil || child.ParentID != root.ID || child.Config.ExecutionPolicy == nil || b.server.executionHosts == nil {
		return false, execution.ErrAuthority
	}
	if root.Config.Execution == nil {
		if root.Checkpoint == nil || root.Checkpoint.WorkerSource == nil {
			return false, execution.ErrAuthority
		}
		resource, err := b.server.executionHosts.Resolve(ctx, root.Actor, b.server.workspaceID, root.Checkpoint.WorkerSource.Reference)
		if err != nil {
			return false, err
		}
		root, err = b.server.agentManager.BindWorkerSource(root.ID, root.Actor, resource)
		if err != nil {
			return false, err
		}
	}
	policy, err := b.server.executionHosts.WorkerCopyPolicy(ctx, root.Actor, b.server.workspaceID, *root.Config.Execution)
	if err != nil {
		return false, err
	}
	prerequisites, err := agents.WorkerInputChanges(root, child)
	if err != nil {
		return false, err
	}
	ref := *root.Config.Execution
	req := execution.WorkingCopyRequest{RequestID: "assignment-" + child.ID, Parent: execution.Binding{Actor: root.Actor, Workspace: b.server.workspaceID, Host: ref.Host, Session: ref.Session, Grant: ref.Grant, Revision: ref.Revision, Job: root.ID}, WorkerRunID: child.ID, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch, Permissions: *child.Config.ExecutionPolicy, MaxBytes: policy.MaxCopyBytes}
	req.Prerequisites = prerequisites
	req.Integrated = agents.WorkerIntegratedInputs(root)
	req.Purpose = child.Config.ExecutionWorkerMode
	reserved := false
	if root.Checkpoint != nil && root.Checkpoint.StorageBudget != nil {
		for _, previous := range root.Checkpoint.StorageBudget.Reservations {
			if previous.Request.WorkerRunID == child.ID {
				if previous.Request.InstructionVersion != root.InstructionVersion || previous.Request.ControlEpoch != root.ControlEpoch {
					return false, execution.ErrConflict
				}
				req, reserved = previous.Request, true
				break
			}
		}
	}
	var status execution.WorkerPreparationStatus
	if reserved {
		status, err = b.InspectExecutionWorker(ctx, root, req)
	} else {
		status, err = b.BeginExecutionWorker(ctx, root, req)
	}
	if err != nil {
		return false, err
	}
	switch status.State {
	case "accepted":
		return false, nil
	case "ready":
		_, err = b.AdmitExecutionWorker(ctx, root, req)
		return err == nil, err
	default:
		return false, fmt.Errorf("worker preparation %s", status.State)
	}
}

// These controller methods are not model tools or HTTP resources. Automatic
// execution-worker admission remains gated on assignment/integration support.
func (b *browserRunTools) BeginExecutionWorker(ctx context.Context, root *agents.Task, req execution.WorkingCopyRequest) (execution.WorkerPreparationStatus, error) {
	if root == nil || root.Config.Execution == nil || b.server.executionHosts == nil {
		return execution.WorkerPreparationStatus{}, execution.ErrAuthority
	}
	// Pin the initial service policy with the work; neither a worker allocation
	// nor a new attempt can silently increase this root allowance.
	if err := b.server.agentManager.ReserveWorkerStorage(root.ID, root.Actor, req, execution.DefaultWorkingCopyPolicy().MaxTotalBytes); err != nil {
		return execution.WorkerPreparationStatus{}, err
	}
	if err := b.server.executionHosts.Admit(ctx, root.Actor, b.server.workspaceID, root.ID, *root.Config.Execution); err != nil {
		return execution.WorkerPreparationStatus{}, err
	}
	status, err := b.server.executionHosts.BeginPreparedWorker(ctx, root.Actor, b.server.workspaceID, *root.Config.Execution, req)
	if err != nil {
		return execution.WorkerPreparationStatus{}, err
	}
	return b.recordWorkerPreparation(root, req, status)
}

func (b *browserRunTools) InspectExecutionWorker(ctx context.Context, root *agents.Task, req execution.WorkingCopyRequest) (execution.WorkerPreparationStatus, error) {
	if root == nil || root.Config.Execution == nil || b.server.executionHosts == nil {
		return execution.WorkerPreparationStatus{}, execution.ErrAuthority
	}
	status, err := b.server.executionHosts.InspectPreparedWorker(ctx, root.Actor, b.server.workspaceID, *root.Config.Execution, req)
	if err != nil {
		return execution.WorkerPreparationStatus{}, err
	}
	return b.recordWorkerPreparation(root, req, status)
}

func (b *browserRunTools) recordWorkerPreparation(root *agents.Task, req execution.WorkingCopyRequest, status execution.WorkerPreparationStatus) (execution.WorkerPreparationStatus, error) {
	if status.Record != nil {
		if err := b.server.agentManager.RecordWorkerStorage(root.ID, root.Actor, req, *status.Record); err != nil {
			return execution.WorkerPreparationStatus{}, err
		}
	}
	return status, nil
}

// Admission is explicit, separately from observation. Recheck the current
// canonical assignment before publishing the already narrowed child resource.
func (b *browserRunTools) AdmitExecutionWorker(ctx context.Context, root *agents.Task, req execution.WorkingCopyRequest) (execution.Resource, error) {
	if root == nil || root.Config.Execution == nil || b.server.executionHosts == nil {
		return execution.Resource{}, execution.ErrAuthority
	}
	if err := b.server.agentManager.ReserveWorkerStorage(root.ID, root.Actor, req, execution.DefaultWorkingCopyPolicy().MaxTotalBytes); err != nil {
		return execution.Resource{}, err
	}
	status, err := b.InspectExecutionWorker(ctx, root, req)
	if err != nil {
		return execution.Resource{}, err
	}
	if status.State != "ready" {
		return execution.Resource{}, execution.ErrHostUnavailable
	}
	resource, err := b.server.executionHosts.AdmitPreparedWorker(ctx, root.Actor, b.server.workspaceID, *root.Config.Execution, req)
	if err != nil {
		return execution.Resource{}, err
	}
	if err = b.server.agentManager.BindWorkerEnvironment(root.ID, root.Actor, req, resource); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// A failed canonical bind never leaves a usable service-owned child.
		_ = b.server.executionHosts.CancelIfAdmitted(cleanup, root.Actor, b.server.workspaceID, req.WorkerRunID, resource.Reference)
		return execution.Resource{}, err
	}
	return resource, nil
}
