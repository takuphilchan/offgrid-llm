package agents

import "time"

// Archival changes presentation, never the recorded effects or execution state.
func CanArchiveTask(t *Task) bool {
	if t == nil || t.DeletedAt != nil || t.ParentID != "" || t.AdmissionHeld || t.PendingApproval != nil {
		return false
	}
	switch t.Status {
	case TaskUncertain, TaskInterrupted, TaskCompleted, TaskFailed, TaskCancelled:
		return true
	}
	return false
}

func (r *Runner) Archive(id, actor string, archived bool) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, active := r.active[id]; active || r.closing {
		return nil, ErrRunConflict
	}
	task, err := r.manager.updateTask(id, func(t *Task) error {
		if t.Actor != actor && !(t.Actor == "" && actor == "local-admin") {
			return ErrTaskNotFound
		}
		if !CanArchiveTask(t) {
			return ErrRunConflict
		}
		for _, link := range t.ChildHistory {
			child := r.manager.tasks[link.ID]
			if _, active := r.active[link.ID]; active {
				return ErrRunConflict
			}
			if child != nil && child.DeletedAt == nil {
				copy := *child
				copy.ParentID = ""
				if !CanArchiveTask(&copy) {
					return ErrRunConflict
				}
			}
		}
		if archived && t.ArchivedAt == nil {
			now := time.Now().UTC()
			t.ArchivedAt = &now
		}
		if !archived {
			t.ArchivedAt = nil
		}
		return nil
	})
	if err == nil {
		r.observe(task)
	}
	return task, err
}
