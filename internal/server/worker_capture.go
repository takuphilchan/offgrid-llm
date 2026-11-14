package server

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Capture runs in the bounded preparation controller, never in a model turn or
// GET handler. The root allocation commits before the native capture command.
func (b *browserRunTools) CaptureExecutionAssignment(ctx context.Context, root, child *agents.Task) error {
	if root == nil || child == nil || b.server.executionHosts == nil {
		return execution.ErrAuthority
	}
	var req execution.SealChangeSetRequest
	known := false
	if root.Checkpoint != nil && root.Checkpoint.StorageBudget != nil {
		for _, reservation := range root.Checkpoint.StorageBudget.Reservations {
			if reservation.Request.WorkerRunID == child.ID && reservation.Capture != nil {
				req, known = reservation.Capture.Request, true
				if reservation.Capture.Result != nil {
					return nil
				}
				break
			}
		}
	}
	fresh := false
	if !known {
		var err error
		req, fresh, err = b.server.agentManager.ReserveWorkerCapture(root.ID, root.Actor, child.ID)
		if err != nil {
			return err
		}
	}
	operation := "seal_status"
	if fresh {
		operation = "seal"
	}
	ref := execution.Reference{Host: req.Source.Host, Session: req.Source.Session, Grant: req.Source.Grant, Revision: req.Source.Revision}
	reply, err := b.server.executionHosts.WorkerArtifact(ctx, root.Actor, b.server.workspaceID, ref, execution.WorkerArtifactRequest{Operation: operation, Source: req.Source, Seal: &req})
	if err != nil {
		reply = execution.WorkerArtifactReply{State: "uncertain"}
	}
	if reply.Copy != nil {
		// Observe real growth even when it exceeds the admitted allowance. Such
		// growth blocks further admission; it must not disappear with a rejected
		// capture settlement or reopen budget on the next attempt.
		current, found := b.server.agentManager.GetTask(root.ID)
		if !found || current.Checkpoint == nil || current.Checkpoint.StorageBudget == nil {
			return agents.ErrRunConflict
		}
		for _, reservation := range current.Checkpoint.StorageBudget.Reservations {
			if reservation.Request.WorkerRunID == child.ID && reservation.CopyID == req.CopyID {
				if err = b.server.agentManager.RecordWorkerStorage(root.ID, root.Actor, reservation.Request, *reply.Copy); err != nil {
					return err
				}
				if reply.Copy.ReservedBytes > req.MaxReservedBytes || reply.Copy.MeasuredBytes > req.MaxReservedBytes {
					reply = execution.WorkerArtifactReply{State: "uncertain"}
				}
				break
			}
		}
	}
	return b.server.agentManager.RecordWorkerCapture(root.ID, root.Actor, child.ID, req, reply)
}
