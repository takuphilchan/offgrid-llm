package server

import (
	"context"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func (b *browserRunTools) ReadWorkerIntegrationReceipt(ctx context.Context, task *agents.Task, req execution.IntegrateChangeSetRequest) (execution.WorkerArtifactReply, error) {
	if b.server.executionHosts == nil || req.Source.Actor != task.Actor || req.Source.Job != task.ID || req.Source.Workspace != b.server.workspaceID {
		return execution.WorkerArtifactReply{}, execution.ErrAuthority
	}
	d, _ := execution.Descriptor("workspace_read")
	if !b.server.profileActorCapabilityAllowed(task.Actor, d) {
		return execution.WorkerArtifactReply{}, execution.ErrAuthority
	}
	ref := execution.Reference{Host: req.Source.Host, Session: req.Source.Session, Grant: req.Source.Grant, Revision: req.Source.Revision}
	return b.server.executionHosts.WorkerArtifact(ctx, task.Actor, b.server.workspaceID, ref, execution.WorkerArtifactRequest{Operation: "integration_status", Source: req.Source, Integrate: &req})
}

func (b *browserRunTools) ObserveWorkerIntegrations(ctx context.Context) {
	items := b.server.agentManager.WorkerIntegrationObservations(b.integrationObservationCursor, 16)
	if len(items) == 0 {
		b.integrationObservationCursor = ""
		return
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		b.integrationObservationCursor = item.Key
		task, found := b.server.agentManager.GetTask(item.RootID)
		if !found || task.Actor != item.Actor {
			continue
		}
		readctx, cancel := context.WithTimeout(ctx, time.Second)
		reply, err := b.ReadWorkerIntegrationReceipt(readctx, task, item.Request)
		cancel()
		if err != nil || reply.State == "accepted" || reply.State == "uncertain" {
			continue
		}
		// Observation updates evidence/accounting only, including after Stop.
		// Reconciliation and further execution still require explicit controls.
		_ = b.server.agentManager.RecordWorkerIntegration(item.RootID, item.Actor, item.WorkerID, item.Request, reply)
	}
}
