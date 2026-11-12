package agents

func (r *Runner) fail(task *Task, message string) (*Task, error) {
	settled, err := r.stopOwnedExecution(task)
	if err != nil || settled.Status == TaskUncertain {
		return settled, err
	}
	finishTask(task, TaskFailed, message)
	if err := r.persist(task); err != nil {
		return nil, err
	}
	return task, nil
}
