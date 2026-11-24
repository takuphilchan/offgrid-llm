package agents

import "testing"

func TestModelAuxiliaryBudgetVersionPublishedAtAdmission(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	saved, _ := m.GetTask(task.ID)
	if saved.Checkpoint.ModelBudget == nil || saved.Checkpoint.ModelBudget.Version != 2 || saved.Checkpoint.ModelBudget.AuxiliaryVersion != 1 {
		t.Fatal("new adaptive task exposed without downgrade refusal", saved.Checkpoint.ModelBudget)
	}
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ = reloaded.GetTask(task.ID)
	if saved.Checkpoint.ModelBudget.Version != 2 {
		t.Fatal("restart lost auxiliary-aware budget version")
	}
}

func TestModelAuxiliaryLegacyVersionMigratesWithoutRefund(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	var err error
	task, err = m.updateTask(task.ID, func(next *Task) error {
		next.Checkpoint.AuxiliaryModelRepairs = nil
		next.Checkpoint.DecisionRepairs = 1
		next.Checkpoint.ModelBudget = &ModelGraphBudget{Version: 1, Switches: 1, Repairs: map[string]int{next.ID: 1}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.persist(task); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Checkpoint.ModelBudget.Version != 2 || saved.Checkpoint.ModelBudget.Switches != 1 || auxiliaryCount(t, m, task.ID) != 1 {
		t.Fatal("migration lost old aggregate or downgrade refusal", saved.Checkpoint.ModelBudget)
	}
}

func TestModelAuxiliaryRootVersionRejectsAmbiguousCombinations(t *testing.T) {
	for _, budget := range []*ModelGraphBudget{
		{Version: 1, AuxiliaryVersion: 1},
		{Version: 1, AuxiliaryRepairs: map[string]int{"run": 1}, Repairs: map[string]int{"run": 1}},
		{Version: 2},
		{Version: 2, AuxiliaryVersion: 2},
		{Version: 3, AuxiliaryVersion: 1},
	} {
		if _, err := budget.corrections(); err == nil {
			t.Fatal("invalid root version combination accepted", budget)
		}
	}
}
