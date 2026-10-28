package server

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func (b *browserRunTools) ReadExecutionReceipt(ctx context.Context, task *agents.Task) (execution.Response, error) {
	if task.Config.Execution == nil || task.Checkpoint == nil || task.Checkpoint.ExecutionIntent == nil || b.server.executionHosts == nil {
		return execution.Response{}, execution.ErrAuthority
	}
	d, _ := execution.Descriptor("workspace_read")
	if !b.server.profileActorCapabilityAllowed(task.Actor, d) {
		return execution.Response{}, execution.ErrAuthority
	}
	resource, err := b.server.executionHosts.Resolve(ctx, task.Actor, b.server.workspaceID, *task.Config.Execution)
	if err != nil || !executionScopeMatches(task, resource) {
		return execution.Response{}, execution.ErrHostUnavailable
	}
	return b.server.executionHosts.InspectReceipt(ctx, task.Actor, b.server.workspaceID, *task.Config.Execution, task.Checkpoint.ExecutionIntent.Request)
}
