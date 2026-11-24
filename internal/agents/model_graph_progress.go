package agents

func (m *Manager) saveDelegationModelProgress(current, next *Task, batch []*Task) error {
	resolveAuxiliaryModelRepair(next, "delegation_admission")
	return m.saveRootModelProgress(current, next, batch)
}

func (m *Manager) saveRootModelProgress(current, next *Task, batch []*Task) error {
	if err := m.acceptRootModelProgress(current, next); err != nil {
		return err
	}
	return m.saveTaskBatch(batch)
}

// Caller holds manager.mu and commits next with its existing task transaction.
// This is for accepted root runtime operations that bypass runner persistence.
// Always detach the canonical budget: admission failure must refund nothing.
func (m *Manager) acceptRootModelProgress(current, next *Task) error {
	if current == nil || current.ParentID != "" || current.ID != next.ID || current.Checkpoint == nil || next.Checkpoint == nil {
		return ErrRunConflict
	}
	if err := validateAuxiliaryModelTransition(current, next); err != nil {
		return err
	}
	next.Checkpoint.DecisionRepairs = 0
	if modelGraphEnabled(current) {
		budget, err := m.initialModelGraphBudget(current)
		if err != nil {
			return err
		}
		if err := budget.record(next, true); err != nil {
			return err
		}
		next.Checkpoint.ModelBudget = budget
	}
	return nil
}
