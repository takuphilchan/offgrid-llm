package agents

func (r *Runner) fail(task *Task, message string) (*Task, error) {
	if err := r.conversationInputBoundary(task); err != nil && task.Checkpoint != nil && task.Checkpoint.ExecutingCall == "" {
		return nil, err
	}
	settled, err := r.stopOwnedExecution(task)
	if err != nil || settled.Status == TaskUncertain {
		return settled, err
	}
	finishTask(task, TaskFailed, message)
	if task.Checkpoint != nil && task.Checkpoint.PartialSynthesis && task.ErrorCode == "" {
		task.ErrorCode = "delegation_partial"
	}
	endPartialSynthesis(task)
	if err := r.persist(task); err != nil {
		return nil, err
	}
	return task, nil
}
