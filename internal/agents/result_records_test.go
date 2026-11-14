package agents

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCanonicalResultIdentityIncludesInstructionsOutcomeAndSources(t *testing.T) {
	base := &Task{ID: "run-fixture", Result: "Same answer", Status: TaskFailed, InstructionVersion: 1, ControlEpoch: 1, Config: AgentConfig{ResultIdentityVersion: 1}}
	initial := resultReference(base)
	for _, change := range []func(*Task){func(t *Task) { t.InstructionVersion++ }, func(t *Task) { t.ControlEpoch++ }, func(t *Task) { t.Status = TaskCompleted }, func(t *Task) { t.Config.KnowledgeDependencies = []string{"different-source"} }} {
		copy, _ := copyTask(base)
		change(copy)
		if resultReference(copy) == initial {
			t.Fatal("equal text concealed different result provenance")
		}
	}
	base.Config.ResultIdentityVersion = 0
	if resultReference(base) != fmt.Sprintf("result:%s:%x", base.ID, sha256.Sum256([]byte(base.Result))) {
		t.Fatal("historical result identity changed")
	}
}

func TestCanonicalResultsRetainExactBytesAndSourceProvenance(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	defer r.Shutdown(context.Background())
	child, _ = m.GetTask(child.ID)
	original := resultReference(child)
	first, err := m.ResultRecord(scope, original)
	if err != nil || first.Content != child.Result || first.WorkerID != child.Worker.SessionID || first.WorkID != root.WorkID || first.IndependentlyVerified {
		t.Fatalf("missing sealed provenance: %+v %v", first, err)
	}
	updated, err := m.updateTask(child.ID, func(task *Task) error {
		task.Result = "A later output with citation [2]."
		task.Config.KnowledgeDependencies = []string{"private-source"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	second := resultReference(updated)
	if _, err = m.updateTask(child.ID, func(task *Task) error { task.Config.KnowledgeDependencies = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	page, err := r.historyPage(context.Background(), root, original, 0)
	if err != nil || !strings.Contains(page, first.Content) {
		t.Fatal("old reference changed", page, err)
	}
	r.SourceAccess = func(task *Task) bool { return !containsTool(task.Config.KnowledgeDependencies, "private-source") }
	if _, err = r.historyPage(context.Background(), root, second, 0); !errors.Is(err, ErrSourceAccess) {
		t.Fatal("mutable run lost result access dependencies", err)
	}
	for _, foreign := range []ControlScope{{Workspace: scope.Workspace, Actor: "other"}, {Workspace: "other", Actor: scope.Actor}} {
		if _, err := m.ResultRecord(foreign, original); !errors.Is(err, ErrControlNotFound) {
			t.Fatal("cross-scope result", err)
		}
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	retained, err := reloaded.ResultRecord(scope, original)
	if err != nil || retained.Content != first.Content {
		t.Fatal("restart lost sealed output", err)
	}
}

func TestCanonicalResultPersistenceSharesTaskTransaction(t *testing.T) {
	m, r, scope, conversation := controlFixture(t)
	defer r.Shutdown(context.Background())
	work := admitControlTestWork(t, r, scope, conversation)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_result BEFORE INSERT ON agent_control_records WHEN NEW.kind='result' BEGIN SELECT RAISE(ABORT,'fixture receipt write failed'); END`); err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(work.RunID, func(task *Task) error {
		finishTask(task, TaskCompleted, "")
		task.Result = "Must not appear without receipt"
		return nil
	})
	if err == nil {
		t.Fatal("result receipt fault accepted")
	}
	var raw string
	if err = db.QueryRow(`SELECT snapshot FROM agent_tasks WHERE id=?`, work.RunID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "Must not appear without receipt") {
		t.Fatal("task committed before result")
	}
	if err = db.QueryRow(`SELECT snapshot FROM agent_control_records WHERE id=?`, work.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, `"status":"completed"`) {
		t.Fatal("work committed before result")
	}
}

func TestCanonicalResultUpgradeBackfillsWithoutDispatch(t *testing.T) {
	m, r, scope, _, child := completedWorkerWork(t)
	defer r.Shutdown(context.Background())
	child, _ = m.GetTask(child.ID)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DELETE FROM agent_control_outbox WHERE event_id IN (SELECT sequence FROM agent_control_events WHERE kind='result_sealed')`,
		`DELETE FROM agent_control_events WHERE kind='result_sealed'`,
		`DELETE FROM agent_control_records WHERE kind='result'`,
		`UPDATE agent_schema SET version=24`,
	} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	result, err := reloaded.ResultRecord(scope, resultReference(child))
	if err != nil || result.Content != child.Result {
		t.Fatal("upgrade lost existing output", err)
	}
	stored, _ := reloaded.GetTask(child.ID)
	if stored.Status != TaskCompleted || stored.SchedulerReady {
		t.Fatal("upgrade dispatched settled work")
	}
	events, err := reloaded.ControlEvents(scope, result.ID, "result", 0, 100)
	if err != nil || len(events.Events) != 1 {
		t.Fatal("missing or duplicate result event", err)
	}
}
