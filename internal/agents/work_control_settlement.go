package agents

// The active runner's teardown and process-termination receipt trigger this
// bookkeeping. A cancelled model may take time to acknowledge; accepted Stop
// stays observable as stopping until every affected operation has settled.
func (r *Runner) settleWorkControls() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	settled := map[string]*Task{}
	for id, task := range r.manager.tasks {
		if !task.ControlPending || r.active[id] != nil || task.ProcessStopPending {
			continue
		}
		pending := false
		for _, link := range task.ChildHistory {
			child := r.manager.tasks[link.ID]
			if child != nil && child.ParentID == task.ID && child.Actor == task.Actor && (r.active[child.ID] != nil || child.ProcessStopPending) {
				pending = true
				break
			}
		}
		if pending {
			continue
		}
		next, err := copyTask(task)
		if err != nil {
			return
		}
		next.ControlPending = false
		settled[id] = next
	}
	if len(settled) == 0 {
		return
	}
	batch := make([]*Task, 0, len(settled))
	for _, task := range settled {
		batch = append(batch, task)
	}
	if r.manager.saveTaskBatch(batch) != nil {
		return
	}
	for id, task := range settled {
		r.manager.tasks[id] = task
	}
}
