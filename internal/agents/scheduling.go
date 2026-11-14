package agents

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func taskSchedulingContext(ctx context.Context, task *Task) context.Context {
	return inference.WithSchedulingScope(ctx, taskSchedulingScope(task))
}

func taskSchedulingScope(task *Task) inference.SchedulingScope {
	work := task.WorkID
	if task.Worker != nil {
		work = task.Worker.WorkID
	}
	if work == "" {
		work = task.ParentID
	}
	if work == "" {
		work = task.ID
	}
	if task.ConversationTurn != nil && task.Origin != nil {
		work = task.Origin.Conversation
	}
	return inference.SchedulingScope{Actor: task.Actor, Work: work, Background: task.ConversationTurn == nil}
}
