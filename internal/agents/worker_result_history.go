package agents

// A new run can retain a worker session without copying old children into its
// current graph. Authorize through exact saved session/run membership; sharing
// an actor or installation does not grant access to unrelated work.
func (r *Runner) retainedWorkerResultAccess(caller, target *Task, reference string) (bool, bool) {
	if target.Worker == nil || caller.Actor != target.Actor {
		return false, false
	}
	var workspace, work string
	known := false
	if caller.ParentID == "" && caller.Origin != nil {
		workspace, work = caller.Origin.Workspace, caller.WorkID
		known = containsTool(caller.PreviousWorkerIDs, target.Worker.SessionID)
		for _, link := range caller.ChildHistory {
			known = known || link.WorkerID == target.Worker.SessionID
		}
	} else if caller.Worker != nil {
		workspace, work = caller.Worker.Workspace, caller.Worker.WorkID
		known = caller.Worker.SessionID == target.Worker.SessionID
		for _, pinned := range caller.DependencyResults {
			known = known || pinned == reference
		}
	}
	if !known || work == "" || workspace == "" || target.Worker.WorkID != work || target.Worker.Workspace != workspace {
		return false, false
	}
	worker, err := r.manager.WorkerSession(ControlScope{Workspace: workspace, Actor: caller.Actor}, target.Worker.SessionID)
	if err != nil || worker.WorkID != work || !containsTool(worker.RunIDs, target.ID) {
		return false, false
	}
	return true, worker.CurrentRunID != target.ID
}
