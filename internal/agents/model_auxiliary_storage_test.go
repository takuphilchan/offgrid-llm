package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelAuxiliaryLegacyAmbiguityPausesButCompletedHistoryDoesNot(t *testing.T) {
	for _, completed := range []bool{false, true} {
		m, r, root, children := modelGraphFixture(t)
		_, err := m.updateTask(children[0].ID, func(next *Task) error {
			next.Checkpoint.AuxiliaryModelRepairs = nil
			next.Checkpoint.SkillAnswerRepairs = 1
			if completed {
				finishTask(next, TaskCompleted, "")
			} else {
				next.Status = TaskInterrupted
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = m.updateTask(root.ID, func(next *Task) error {
			next.Checkpoint.ModelBudget.Version, next.Checkpoint.ModelBudget.AuxiliaryVersion = 1, 0
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
		if err := m.StorageError(); err != nil {
			t.Fatal(err)
		}
		root, _ = m.GetTask(root.ID)
		budget, err := m.initialModelGraphBudget(root)
		if err != nil {
			t.Fatal(err)
		}
		count, err := budget.corrections()
		want := 3
		if completed {
			want = 0
		}
		if err != nil || count != want {
			t.Fatal("legacy history misclassified", completed, count, err)
		}
		if completed {
			continue
		}
		calls := 0
		r = NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			calls++
			return nil, errors.New("forbidden inference")
		})
		if _, err = m.updateTask(root.ID, func(next *Task) error { next.Status = TaskChildren; return nil }); err != nil {
			t.Fatal(err)
		}
		paused, err := r.Continue(context.Background(), children[0].ID, root.Actor, "resume", "", false)
		if err != nil || calls != 0 || paused.Status != TaskInterrupted || !strings.Contains(paused.Error, "older checkpoint") {
			t.Fatalf("ambiguous legacy state inferred success: calls=%d task=%+v error=%v", calls, paused, err)
		}
	}
}

func TestModelAuxiliaryStoredUnknownStateFailsClosed(t *testing.T) {
	for _, kind := range []string{"version", "category", "negative", "sequence_negative", "witness_category", "witness_negative", "projection", "projection_version"} {
		t.Run(kind, func(t *testing.T) {
			m, r, task := auxiliaryFixture(t)
			if err := r.persist(task); err != nil {
				t.Fatal(err)
			}
			next, err := copyTask(task)
			if err != nil {
				t.Fatal(err)
			}
			func() {
				switch kind {
				case "version":
					next.Checkpoint.AuxiliaryModelRepairs.Version = 99
				case "category":
					next.Checkpoint.AuxiliaryModelRepairs.Active = map[string]int{"model_claimed_resolution": 1}
				case "negative":
					next.Checkpoint.AuxiliaryModelRepairs.Active = map[string]int{"skill_answer": -1}
				case "sequence_negative":
					next.Checkpoint.AuxiliaryModelRepairs.Rejections = -1
				case "witness_category":
					next.Checkpoint.AuxiliaryModelRepairs.Resolved = map[string]int{"model_claimed_resolution": 1}
				case "witness_negative":
					next.Checkpoint.AuxiliaryModelRepairs.Resolved = map[string]int{"skill_answer": -1}
				case "projection":
					next.Checkpoint.ModelBudget.AuxiliaryRepairs = map[string]int{next.ID: 1}
				case "projection_version":
					next.Checkpoint.ModelBudget.AuxiliaryVersion = 99
				}
			}()
			if err = validateStoredTask(next); err == nil {
				t.Fatal("invalid accounting accepted for storage")
			}
			data, err := json.Marshal(next)
			if err != nil {
				t.Fatal(err)
			}
			db, err := openTaskDatabase(m.dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err = db.Exec(`UPDATE agent_tasks SET snapshot=? WHERE id=?`, data, next.ID); err != nil {
				t.Fatal(err)
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if reloaded.StorageError() == nil {
				t.Fatal("invalid persisted accounting accepted")
			}
		})
	}
}

func TestModelAuxiliaryChargeRollsBackWithRootAndRetainsRestartDebt(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	child, err := m.updateTask(children[0].ID, func(next *Task) error { next.Status = TaskRunning; attachContractFixture(next); return nil })
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_auxiliary BEFORE UPDATE ON agent_tasks WHEN NEW.id='` + root.ID + `' BEGIN SELECT RAISE(ABORT,'fixture auxiliary rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.repairSkillAnswer(child, "incomplete"); err == nil {
		t.Fatal("failed root transaction accepted")
	}
	saved, _ := m.GetTask(child.ID)
	if saved.Checkpoint.SkillAnswerRepairs != 0 || len(saved.Checkpoint.AuxiliaryModelRepairs.Active) != 0 || auxiliaryCount(t, m, root.ID) != 0 {
		t.Fatal("partially published auxiliary debt")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_auxiliary`); err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(root.ID, func(next *Task) error { next.Status = TaskChildren; return nil }); err != nil {
		t.Fatal(err)
	}
	child, err = m.updateTask(child.ID, func(next *Task) error { next.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	r = NewRunner(m, &runTestTools{}, nil)
	if _, err = r.repairSkillAnswer(child, "incomplete"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER reject_auxiliary_resolution BEFORE UPDATE ON agent_tasks WHEN NEW.id='` + root.ID + `' BEGIN SELECT RAISE(ABORT,'fixture auxiliary resolution rollback'); END`); err != nil {
		t.Fatal(err)
	}
	finishTask(child, TaskCompleted, "")
	if err = r.persistModelProgress(child); err == nil {
		t.Fatal("failed resolution transaction accepted")
	}
	canonical, _ := m.GetTask(child.ID)
	if canonical.Status != TaskRunning || canonical.Checkpoint.AuxiliaryModelRepairs.Active["skill_answer"] != 1 || len(canonical.Checkpoint.AuxiliaryModelRepairs.Resolved) != 0 {
		t.Fatal("failed resolution partially published")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_auxiliary_resolution`); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if reloaded.StorageError() != nil || auxiliaryCount(t, reloaded, root.ID) != 1 {
		t.Fatal("durable auxiliary debt lost")
	}
}

func TestModelAuxiliaryRejectionHonorsPendingControl(t *testing.T) {
	for _, boundary := range []string{"steering", "inbox", "stop", "parent_stop"} {
		t.Run(boundary, func(t *testing.T) {
			m, r, root, children := modelGraphFixture(t)
			child, err := m.updateTask(children[0].ID, func(next *Task) error { next.Status = TaskRunning; attachContractFixture(next); return nil })
			if err != nil {
				t.Fatal(err)
			}
			id := child.ID
			if boundary == "parent_stop" {
				id = root.ID
			}
			_, err = m.updateTask(id, func(next *Task) error {
				switch boundary {
				case "steering":
					next.PendingInstructions = []Instruction{{Text: "Preserve revised constraints"}}
				case "inbox":
					return holdForConversationInput(next, "new-input")
				default:
					next.Status = TaskCancelled
					next.ControlAction = "stop"
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.repairSkillAnswer(child, "incomplete"); err == nil {
				t.Fatal("stale rejection crossed control fence")
			}
			saved, _ := m.GetTask(child.ID)
			if saved.Checkpoint.SkillAnswerRepairs != 0 || len(saved.Checkpoint.AuxiliaryModelRepairs.Active) != 0 || auxiliaryCount(t, m, root.ID) != 0 {
				t.Fatal("superseded rejection was charged")
			}
		})
	}
}

func TestModelAuxiliaryContinuationTransfersOnceWithoutExecutableState(t *testing.T) {
	m, r, root, old := stoppedModelWorker(t)
	_, err := m.updateTask(old.ID, func(next *Task) error {
		chargeAuxiliaryModelRepair(next, "skill_answer")
		next.Checkpoint.SkillAnswerRepairs = 1
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(root.ID, func(next *Task) error {
		next.Checkpoint.ModelBudget.Repairs[old.ID] = 2
		next.Checkpoint.ModelBudget.AuxiliaryRepairs = map[string]int{old.ID: 1}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	worker, _ := m.WorkerSession(scope, old.Worker.SessionID)
	receipt, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "auxiliary-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Review the revised request"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(receipt.Worker.CurrentRunID)
	parent, _ := m.GetTask(root.ID)
	if next.Checkpoint.DecisionRepairs != 2 || len(next.Checkpoint.AuxiliaryModelRepairs.Active) != 0 || len(next.Checkpoint.Calls) != 0 || next.Checkpoint.ExecutionReceipt != nil || parent.Checkpoint.ModelBudget.Repairs[old.ID] != 0 || parent.Checkpoint.ModelBudget.AuxiliaryRepairs[old.ID] != 0 {
		t.Fatal("continuation lost, duplicated or replayed debt")
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if reloaded.StorageError() != nil || auxiliaryCount(t, reloaded, root.ID) != 2 {
		t.Fatal("restart changed transferred auxiliary debt")
	}
}
