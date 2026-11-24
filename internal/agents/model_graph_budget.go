package agents

import (
	"context"
	"fmt"
)

// ModelGraphBudget belongs to the root. Repair entries are unresolved sequences
// per run: success elsewhere cannot erase them. Switches are never refunded.
// It is committed with a child's checkpoint in the existing task transaction.
type ModelGraphBudget struct {
	Version          int            `json:"version"`
	AuxiliaryVersion int            `json:"auxiliary_version,omitempty"`
	Switches         int            `json:"switches"`
	Repairs          map[string]int `json:"repairs,omitempty"`
	AuxiliaryRepairs map[string]int `json:"auxiliary_repairs,omitempty"`
}

func (b *ModelGraphBudget) corrections() (int, error) {
	if b == nil || !(b.Version == 1 && b.AuxiliaryVersion == 0 && len(b.AuxiliaryRepairs) == 0 || b.Version == 2 && b.AuxiliaryVersion == 1) || b.Switches < 0 || b.Switches > maxModelSwitches || len(b.Repairs) > 10000 {
		return 0, fmt.Errorf("invalid root model budget")
	}
	total := 0
	for id, count := range b.Repairs {
		if id == "" || count < 1 || count > 100 {
			return 0, fmt.Errorf("invalid root corrective sequence")
		}
		total += count
	}
	for id, count := range b.AuxiliaryRepairs {
		if b.AuxiliaryVersion != 1 || count < 1 || count > b.Repairs[id] {
			return 0, fmt.Errorf("invalid root auxiliary corrective sequence")
		}
	}
	return total, nil
}

// Version 2 is deliberately not understood by the earlier aggregate-only
// reader: downgrading must refuse these ledgers, not discard auxiliary debt.
func newModelGraphBudget() *ModelGraphBudget {
	return &ModelGraphBudget{Version: 2, AuxiliaryVersion: 1, Repairs: map[string]int{}, AuxiliaryRepairs: map[string]int{}}
}

func modelGraphEnabled(root *Task) bool {
	if root.Checkpoint.ModelBudget != nil || root.Config.ModelSelection != nil && root.Config.ModelSelection.Mode == "adaptive" {
		return true
	}
	for _, child := range root.Config.DelegateProfiles {
		if child.ModelSelection != nil && child.ModelSelection.Mode == "adaptive" {
			return true
		}
	}
	return false
}

// Caller holds manager.mu. Only the established direct ownership graph can
// participate; model-provided identities cannot select a different root budget.
func (m *Manager) modelGraphRoot(task *Task) (*Task, error) {
	if task.ParentID == "" {
		return task, nil
	}
	root := m.tasks[task.ParentID]
	if root == nil || root.DeletedAt != nil || root.ParentID != "" || root.Actor != task.Actor || root.Checkpoint == nil {
		return nil, ErrRunConflict
	}
	for _, link := range root.ChildHistory {
		if link.ID == task.ID {
			return root, nil
		}
	}
	return nil, ErrRunConflict
}

func (m *Manager) initialModelGraphBudget(root *Task) (*ModelGraphBudget, error) {
	b := newModelGraphBudget()
	if saved := root.Checkpoint.ModelBudget; saved != nil {
		if _, err := saved.corrections(); err != nil {
			return nil, err
		}
		b.Switches = saved.Switches
		for id, count := range saved.Repairs {
			b.Repairs[id] = count
		}
		for id, count := range saved.AuxiliaryRepairs {
			b.AuxiliaryRepairs[id] = count
		}
		if saved.Version == 2 {
			return b, nil
		}
	}
	// Existing admitted work has no aggregate record. Seed it from its durable
	// history instead of granting fresh switches or discarding spent repairs.
	members := []*Task{root}
	for _, link := range root.ChildHistory {
		child := m.tasks[link.ID]
		if child == nil || child.ParentID != root.ID || child.Actor != root.Actor || child.Checkpoint == nil {
			return nil, ErrRunStorage
		}
		members = append(members, child)
	}
	for _, task := range members {
		if root.Checkpoint.ModelBudget == nil {
			b.Switches += len(task.Checkpoint.ModelSwitches)
		}
		if err := b.record(task, false); err != nil {
			return nil, err
		}
	}
	_, err := b.corrections()
	return b, err
}

func (b *ModelGraphBudget) record(task *Task, resolved bool) error {
	auxiliary, err := auxiliaryModelCorrections(task)
	if err != nil {
		return err
	}
	base := task.Checkpoint.DecisionRepairs + task.Checkpoint.ProposalRepairs
	if !resolved {
		base = max(base, b.Repairs[task.ID]-b.AuxiliaryRepairs[task.ID])
	}
	count := base + auxiliary
	if b.AuxiliaryRepairs == nil {
		b.AuxiliaryRepairs = map[string]int{}
	}
	if auxiliary == 0 {
		delete(b.AuxiliaryRepairs, task.ID)
	} else {
		b.AuxiliaryRepairs[task.ID] = auxiliary
	}
	if count == 0 {
		delete(b.Repairs, task.ID)
	} else {
		b.Repairs[task.ID] = count
	}
	return nil
}

// All runner checkpoint publication goes through this lock. Concurrent sibling
// rejections therefore cannot each reserve the last shared fallback switch.
func (m *Manager) updateModelGraph(id string, rejected bool, change func(*Task) (bool, error)) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.tasks[id]
	if current == nil || current.DeletedAt != nil {
		return nil, ErrTaskNotFound
	}
	next, err := copyTask(current)
	if err != nil {
		return nil, err
	}
	accepted, err := change(next)
	if err != nil {
		return nil, err
	}
	_, err = auxiliaryModelCorrections(next)
	if err != nil {
		return nil, err
	}
	_, err = auxiliaryModelCorrections(current)
	if err != nil {
		return nil, err
	}
	if err := validateAuxiliaryModelTransition(current, next); err != nil {
		return nil, err
	}
	auxiliaryRejected := auxiliaryModelRejection(current, next)
	rejected = rejected && next.Checkpoint.DecisionRepairs == current.Checkpoint.DecisionRepairs+1 && next.Checkpoint.Iteration == current.Checkpoint.Iteration+1
	root, err := m.modelGraphRoot(current)
	if err != nil {
		return nil, err
	}
	batch := []*Task{next}
	if modelGraphEnabled(root) {
		budget, err := m.initialModelGraphBudget(root)
		if err != nil {
			return nil, err
		}
		if (rejected || auxiliaryRejected) && current.ParentID != "" && (root.Status != TaskChildren || root.ControlPending || root.ControlAction == "stop" || hasConversationHold(root) || len(root.PendingInstructions) != 0) {
			return nil, context.Canceled
		}
		// Queued proposals and failed invocations also advance/clear checkpoint
		// fields. Only an accepted success may reduce unresolved corrections.
		if err := budget.record(next, accepted); err != nil {
			return nil, err
		}
		corrections, err := budget.corrections()
		if err != nil {
			return nil, err
		}
		if corrections >= maxProposalRepairs && next.Checkpoint.BudgetedModelCalls > current.Checkpoint.BudgetedModelCalls {
			return nil, fmt.Errorf("shared model correction budget exhausted before inference reservation")
		}
		if rejected && next.Status == TaskRunning {
			if corrections >= maxProposalRepairs {
				interruptModelGraph(next)
			} else if budget.Switches < maxModelSwitches && modelSwitchBoundarySafe(current) {
				before := len(next.Checkpoint.ModelSwitches)
				adaptRejectedGraphDecision(next, current.ParentID != "")
				budget.Switches += len(next.Checkpoint.ModelSwitches) - before
			}
		}
		owner := next
		if current.ParentID != "" {
			owner, err = copyTask(root)
			if err != nil {
				return nil, err
			}
			next.Checkpoint.ModelBudget = nil
			batch = append(batch, owner)
		}
		owner.Checkpoint.ModelBudget = budget
	} else if rejected {
		adaptRejectedDecision(next)
	}
	if len(batch) == 1 {
		err = m.saveTask(next)
	} else {
		err = m.saveTaskBatch(batch)
	}
	if err != nil {
		return nil, err
	}
	for _, task := range batch {
		m.tasks[task.ID] = task
	}
	return copyTask(next)
}

func interruptModelGraph(task *Task) {
	task.Status, task.ErrorCode = TaskInterrupted, "model_decision_repair_limit"
	task.Error = "The shared corrective budget is exhausted. Parent and worker decisions retain their saved charges; no new inference was started."
	if legacyAuxiliaryAmbiguous(task) && task.Checkpoint.AuxiliaryModelRepairs == nil || task.Checkpoint.AuxiliaryModelRepairs != nil && task.Checkpoint.AuxiliaryModelRepairs.Active["legacy_ambiguous"] > 0 {
		task.Error += " This older checkpoint does not identify which historical corrections were resolved; its allowance is conservatively exhausted pending review."
	}
	setRunPhase(task, "interrupted", "")
}

func (r *Runner) modelGraphBoundary(task *Task) (bool, error) {
	r.manager.mu.RLock()
	current := r.manager.tasks[task.ID]
	if current == nil {
		r.manager.mu.RUnlock()
		return true, ErrTaskNotFound
	}
	root, err := r.manager.modelGraphRoot(current)
	count := 0
	if err == nil && modelGraphEnabled(root) {
		var budget *ModelGraphBudget
		budget, err = r.manager.initialModelGraphBudget(root)
		if err == nil {
			count, err = budget.corrections()
		}
	}
	r.manager.mu.RUnlock()
	if err != nil {
		return true, err
	}
	if count < maxProposalRepairs {
		return false, nil
	}
	interruptModelGraph(task)
	return true, r.persist(task)
}
