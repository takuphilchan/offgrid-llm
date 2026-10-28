package agents

import (
	"context"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

var ErrExecutionEvidenceUnavailable = errors.New("execution evidence unavailable")

func CanRecoverExecution(task *Task) bool {
	return task != nil && task.Status == TaskUncertain && task.Checkpoint != nil && validExecutionIntent(task, task.Checkpoint.ExecutionIntent)
}

// RecoverExecution checks only an already recorded intent. It never accepts a
// replacement request/body and never dispatches or resumes the original action.
func (r *Runner) RecoverExecution(ctx context.Context, id, actor string) (*Task, error) {
	r.mu.Lock()
	if r.closing {
		r.mu.Unlock()
		return nil, ErrRunConflict
	}
	if _, active := r.active[id]; active {
		r.mu.Unlock()
		return nil, ErrRunConflict
	}
	task, ok := r.manager.GetTask(id)
	if !ok || task.Actor != actor {
		r.mu.Unlock()
		return nil, ErrTaskNotFound
	}
	if !CanRecoverExecution(task) {
		r.mu.Unlock()
		return nil, ErrRunConflict
	}
	if recoverExecutionReceipt(task) {
		updated, err := r.manager.updateTask(id, func(current *Task) error {
			if !recoverExecutionReceipt(current) {
				return ErrRunConflict
			}
			return nil
		})
		r.mu.Unlock()
		if err == nil {
			r.observe(updated)
		}
		return updated, err
	}
	reader, ok := r.tools.(interface {
		ReadExecutionReceipt(context.Context, *Task) (execution.Response, error)
	})
	if !ok {
		r.mu.Unlock()
		return task, ErrExecutionEvidenceUnavailable
	}
	readctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	r.active[id] = cancel
	r.workers.Add(1)
	r.mu.Unlock()
	defer r.workers.Done()
	defer func() { cancel(); r.mu.Lock(); delete(r.active, id); r.mu.Unlock() }()
	response, err := reader.ReadExecutionReceipt(readctx, task)
	if err != nil {
		return task, errors.Join(ErrExecutionEvidenceUnavailable, err)
	}
	if err = r.manager.RecordExecutionReceipt(id, actor, task.Checkpoint.ExecutionIntent.Request, response); err != nil {
		return task, err
	}
	updated, err := r.manager.updateTask(id, func(current *Task) error {
		if !recoverExecutionReceipt(current) {
			return ErrExecutionEvidenceUnavailable
		}
		return nil
	})
	if err == nil {
		r.observe(updated)
	}
	return updated, err
}
