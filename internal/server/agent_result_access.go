package server

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

// A graph link is necessary, but never substitutes for current source access.
func (b *browserRunTools) AuthorizeTaskResult(ctx context.Context, caller, result *agents.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if caller.Actor != result.Actor || !b.server.taskKnowledgeAllowed(caller.Actor, result) {
		return capabilities.ErrDenied
	}
	return nil
}
