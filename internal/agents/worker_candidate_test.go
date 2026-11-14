package agents

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestWorkerCandidatePinsOnlyCurrentRecordedDependencies(t *testing.T) {
	m, root, editorID, seal := captureBudgetFixture(t)
	if err := m.RecordWorkerCapture(root.ID, root.Actor, editorID, seal, sealedBudgetReply(seal, editorID)); err != nil {
		t.Fatal(err)
	}
	root, _ = m.GetTask(root.ID)
	editor, _ := m.GetTask(editorID)
	tester, err := copyTask(editor)
	if err != nil {
		t.Fatal(err)
	}
	tester.ID += "-test"
	tester.Worker.SessionID += "-test"
	tester.Status = TaskPending
	tester.Config.Execution, tester.Config.ExecutionWorkspace = nil, nil
	tester.Checkpoint = &Checkpoint{}
	root.Delegation.Children[0].Spec.Mode = "edit"
	key := root.Delegation.Children[0].Spec.Key
	link := ChildLink{ID: tester.ID, WorkerID: tester.Worker.SessionID, Spec: ChildSpec{Key: "test", Mode: "test", Goal: "Validate edit", DependsOn: []string{key}}, Status: TaskPending}
	root.Delegation.Children = append(root.Delegation.Children, link)
	root.ChildHistory = append(root.ChildHistory, link)
	m.mu.Lock()
	err = m.saveTaskBatch([]*Task{root, tester})
	if err == nil {
		m.tasks[root.ID], m.tasks[tester.ID] = root, tester
	}
	m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	refs, err := WorkerPrerequisiteChanges(root, tester.ID)
	if err != nil || len(refs) != 1 {
		t.Fatal(refs, err)
	}
	req := execution.WorkingCopyRequest{RequestID: "test", Parent: seal.Source, WorkerRunID: tester.ID, InstructionVersion: root.InstructionVersion, ControlEpoch: root.ControlEpoch, Permissions: execution.Permissions{Read: true}, MaxBytes: 100, Prerequisites: refs}
	for _, kind := range []string{"missing", "forged", "stale"} {
		bad := req
		bad.Prerequisites = append([]execution.ChangeSetReference(nil), refs...)
		switch kind {
		case "missing":
			bad.Prerequisites = nil
		case "forged":
			bad.Prerequisites[0].ID = strings.Repeat("b", 64)
		case "stale":
			bad.Prerequisites[0].InstructionVersion++
		}
		if err = m.ReserveWorkerStorage(root.ID, root.Actor, bad, 300); err == nil {
			t.Fatal("unrecorded dependency admitted", kind)
		}
	}
	if err = m.ReserveWorkerStorage(root.ID, root.Actor, req, 300); err != nil {
		t.Fatal(err)
	}
	req.Prerequisites[0].ID = strings.Repeat("f", 64)
	stored, _ := m.GetTask(root.ID)
	reservation := stored.Checkpoint.StorageBudget.Reservations[storageReservationKey(req)]
	if reservation.Request.Prerequisites[0].ID == req.Prerequisites[0].ID {
		t.Fatal("caller changed accepted candidate")
	}
	req = reservation.Request
	record := execution.WorkingCopyRecord{ID: storageReservationKey(req), State: "ready", Source: req.Parent, WorkerRunID: tester.ID, ReservedBytes: 100, MeasuredBytes: 20, UsageMeasured: true}
	if err = m.RecordWorkerStorage(root.ID, root.Actor, req, record); err != nil {
		t.Fatal(err)
	}
	if _, err = m.updateTask(root.ID, func(task *Task) error { task.Delegation.Children[0].Invalidated = true; return nil }); err != nil {
		t.Fatal(err)
	}
	resource := execution.Resource{Reference: execution.Reference{Host: req.Parent.Host, Session: "test-session", Grant: "test-grant", Revision: 1}, Backend: "local", Root: "private-test", Permissions: req.Permissions}
	if err = m.BindWorkerEnvironment(root.ID, root.Actor, req, resource); err == nil {
		t.Fatal("invalidated prerequisite admitted after preparation")
	}
}
