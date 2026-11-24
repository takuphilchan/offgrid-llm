package agents

import (
	"errors"
	"testing"
)

func TestModelAuxiliaryStaleOwnSnapshotCannotRefundNewCharge(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		m, r, task := auxiliaryFixture(t)
		if err := r.persist(task); err != nil {
			t.Fatal(err)
		}
		stale, err := copyTask(task)
		if err != nil {
			t.Fatal(err)
		}
		chargeAuxiliaryModelRepair(task, "skill_answer")
		if err = r.persist(task); err != nil {
			t.Fatal(err)
		}
		if accepted {
			finishTask(stale, TaskCompleted, "")
			err = r.persistModelProgress(stale)
		} else {
			err = r.persist(stale)
		}
		if err == nil || auxiliaryCount(t, m, task.ID) != 1 {
			t.Fatalf("stale own snapshot refunded newer auxiliary debt: accepted=%v error=%v count=%d", accepted, err, auxiliaryCount(t, m, task.ID))
		}
	}
}

func TestModelAuxiliaryDecreaseNeedsMatchingWitnessEvenAtAcceptedProgress(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		m, r, task := auxiliaryFixture(t)
		chargeAuxiliaryModelRepair(task, "workflow_completion")
		if err := r.persist(task); err != nil {
			t.Fatal(err)
		}
		delete(task.Checkpoint.AuxiliaryModelRepairs.Active, "workflow_completion")
		var err error
		if accepted {
			err = r.persistModelProgress(task)
		} else {
			err = r.persist(task)
		}
		if !errors.Is(err, ErrRunConflict) || auxiliaryCount(t, m, task.ID) != 1 {
			t.Fatal("unwitnessed auxiliary decrease accepted", accepted, err)
		}
	}
}

func TestModelAuxiliaryStaleRootRuntimeCannotRefundNewCharge(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	stale, err := copyTask(task)
	if err != nil {
		t.Fatal(err)
	}
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	if err = r.persist(task); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	err = m.acceptRootModelProgress(m.tasks[task.ID], stale)
	m.mu.Unlock()
	if !errors.Is(err, ErrRunConflict) || auxiliaryCount(t, m, task.ID) != 1 {
		t.Fatal("stale runtime acceptance refunded debt", err)
	}
}

func TestModelAuxiliarySameCategoryResolutionAndRejectionHonorsSteering(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	if _, err := m.updateTask(task.ID, func(next *Task) error {
		next.PendingInstructions = []Instruction{{Text: "Revised request"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	resolveAuxiliaryModelRepair(task, "workflow_completion")
	chargeAuxiliaryModelRepair(task, "workflow_completion")
	if err := r.persist(task); !errors.Is(err, errSteeringPending) {
		t.Fatal("same-count rejection crossed steering fence", err)
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Checkpoint.AuxiliaryModelRepairs.Rejections != 1 || len(saved.Checkpoint.AuxiliaryModelRepairs.Resolved) != 0 || auxiliaryCount(t, m, task.ID) != 1 {
		t.Fatal("superseded resolution/rejection replaced canonical sequence")
	}
}
