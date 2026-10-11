package computer

import (
	"context"
	"errors"
	"testing"
	"time"
)

// An owned in-memory provider tests the dispatcher/journal boundaries. It is
// not registered as a native provider and cannot qualify UIA/AX/AT-SPI input.
type documentDispatchFixture struct {
	inventory             DocumentInventory
	snapshot              DocumentSnapshot
	inputs, reads         int
	inputError, readError error
	wrongResult           bool
}

func (f *documentDispatchFixture) ReadDocument(ctx context.Context, binding ControlBinding) (DocumentInventory, DocumentSnapshot, error) {
	f.reads++
	if ctx.Err() != nil {
		return DocumentInventory{}, DocumentSnapshot{}, ctx.Err()
	}
	if f.readError != nil {
		return DocumentInventory{}, DocumentSnapshot{}, f.readError
	}
	f.inventory.Observation.ID = nativeID()
	f.inventory.Observation.CapturedAt = time.Now()
	f.inventory.Active.Observation = f.inventory.Observation.ID
	f.snapshot.Binding = *f.inventory.Active
	f.snapshot.ID = nativeID()
	return cloneDocumentInventory(f.inventory), copyDocumentSnapshot(f.snapshot), nil
}
func (f *documentDispatchFixture) ApplyDocumentText(_ context.Context, _ ControlBinding, plan PreparedDocumentText) error {
	f.inputs++
	f.snapshot.Text = plan.ExpectedText()
	selection := plan.ExpectedSelection()
	f.snapshot.Selection = &selection
	f.inventory.Active.Revision = nativeID()
	if f.wrongResult {
		f.snapshot.Text += "wrong"
	}
	return f.inputError
}
func (f *documentDispatchFixture) ApplyDocumentLifecycle(_ context.Context, _ ControlBinding, before DocumentInventory, op DocumentOperation) error {
	f.inputs++
	if op.Kind == "create_document" {
		doc := f.inventory.Documents[0]
		doc.ID, doc.Container, doc.Generation = nativeID(), nativeID(), nativeID()
		for i := range f.inventory.Documents {
			f.inventory.Documents[i].Selected = false
		}
		f.inventory.Documents = append(f.inventory.Documents, doc)
		f.inventory.Active.Document, f.inventory.Active.Container, f.inventory.Active.Generation = doc.ID, doc.Container, doc.Generation
		f.snapshot.Text, f.snapshot.Selection = "", &DocumentTextRange{}
	} else {
		for i := range f.inventory.Documents {
			d := &f.inventory.Documents[i]
			d.Selected = d.ID == op.Document
			if d.Selected {
				f.inventory.Active.Document, f.inventory.Active.Generation, f.inventory.Active.Container = d.ID, d.Generation, d.Container
			}
		}
	}
	if f.wrongResult {
		f.snapshot.Text = "template is not blank"
	}
	return f.inputError
}

func documentDispatchSetup(t *testing.T, kind string) (*NativeDocumentExecutor, *documentDispatchFixture, ControlBinding, PreparedControlAction) {
	t.Helper()
	view, snapshot := documentFixture()
	text := "OffGrid test — café 日本語 12345"
	snapshot.Text, snapshot.Selection = "", &DocumentTextRange{}
	if kind == "replace_selection" {
		snapshot.Text, snapshot.Selection = "keep old suffix", &DocumentTextRange{Start: 5, End: 8}
	}
	f := &documentDispatchFixture{inventory: view, snapshot: snapshot}
	b := ControlBinding{Actor: "actor", Task: "task", Companion: "host", Session: "session", Target: view.Observation.Target}
	e := NewNativeDocumentExecutor(f)
	op := DocumentOperation{Kind: kind, Document: snapshot.Binding.Document, Snapshot: snapshot.ID, Text: &text}
	switch kind {
	case "create_document":
		op = DocumentOperation{Kind: kind, Command: view.Commands[0].ID}
	case "select_document":
		op = DocumentOperation{Kind: kind, Document: view.Documents[0].ID}
	case "select_text":
		op.Text = nil
		op.Range = &DocumentTextRange{}
	}
	a, err := e.Prepare(b, op, view, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return e, f, b, a
}

func TestDocumentDispatcherVerifiesAndConsumesEveryMode(t *testing.T) {
	for _, kind := range []string{"create_document", "select_document", "select_text", "insert_text", "replace_selection", "replace_value"} {
		t.Run(kind, func(t *testing.T) {
			e, f, _, a := documentDispatchSetup(t, kind)
			r, err := e.Dispatch(context.Background(), a)
			if err != nil || r.Outcome != "verified" || r.Uncertain || f.inputs != 1 || f.reads < 2 {
				t.Fatal(r, err, f.inputs, f.reads)
			}
			if kind != "select_document" && kind != "select_text" && (r.Document == nil || r.Document.Validate() != nil || r.Document.ActionID != a.ID) {
				t.Fatal("missing bound native evidence")
			}
			if _, err = e.Dispatch(context.Background(), a); !errors.Is(err, ErrControlApproval) || f.inputs != 1 {
				t.Fatal("replayed input", err)
			}
		})
	}
}

func TestDocumentDispatcherRefusesStaleAndForgedPreparationWithoutInput(t *testing.T) {
	for _, change := range []func(*documentDispatchFixture, *PreparedControlAction){
		func(f *documentDispatchFixture, a *PreparedControlAction) { f.inventory.Active.Revision = "changed" },
		func(f *documentDispatchFixture, a *PreparedControlAction) {
			f.snapshot.Selection = &DocumentTextRange{Start: 1, End: 1}
		},
		func(f *documentDispatchFixture, a *PreparedControlAction) { f.snapshot.Composition = "active" },
		func(f *documentDispatchFixture, a *PreparedControlAction) { f.snapshot.Composition = "unknown" },
		func(f *documentDispatchFixture, a *PreparedControlAction) { *a.Operation.Document.Text = "forged" },
		func(f *documentDispatchFixture, a *PreparedControlAction) {
			f.inventory.Documents[0].Generation = "replaced"
		},
		func(f *documentDispatchFixture, a *PreparedControlAction) { f.inventory.Complete = false },
		func(f *documentDispatchFixture, a *PreparedControlAction) { f.readError = ErrControlScope },
	} {
		e, f, _, a := documentDispatchSetup(t, "insert_text")
		change(f, &a)
		r, err := e.Dispatch(context.Background(), a)
		if err == nil || r.Uncertain || r.Outcome != "failed" || f.inputs != 0 {
			t.Fatal("invalid input dispatched", r, err)
		}
	}
}

func TestDocumentDispatcherPartialInputNeverFallsBack(t *testing.T) {
	for _, kind := range []string{"create_document", "insert_text", "replace_selection", "replace_value"} {
		e, f, _, a := documentDispatchSetup(t, kind)
		f.inputError = errors.New("provider disconnected after input")
		r, err := e.Dispatch(context.Background(), a)
		if !errors.Is(err, ErrUncertain) || !r.Uncertain || f.inputs != 1 || f.reads != 1 {
			t.Fatal(r, err, f.inputs)
		}
		if _, err = e.Dispatch(context.Background(), a); err == nil || f.inputs != 1 {
			t.Fatal("input repeated")
		}
	}
}

func TestDocumentDispatcherDelayedReadbackDoesNotRepeatInput(t *testing.T) {
	e, f, _, a := documentDispatchSetup(t, "insert_text")
	f.wrongResult = true
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	r, err := e.Dispatch(ctx, a)
	if !errors.Is(err, ErrUncertain) || !r.Uncertain || f.inputs != 1 || f.reads < 2 {
		t.Fatal(r, err, f.inputs, f.reads)
	}
}

func TestDocumentDispatcherJournalDoesNotRedispatchAcknowledgedOrLostResults(t *testing.T) {
	for _, lose := range []bool{false, true} {
		e, f, b, a := documentDispatchSetup(t, "insert_text")
		journal, err := OpenNativeJournal(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer journal.Close()
		if _, err = journal.claim(context.Background(), b, a); err != nil {
			t.Fatal(err)
		}
		r, err := e.Dispatch(context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		if !lose {
			if err = journal.complete(context.Background(), r); err != nil {
				t.Fatal(err)
			}
		}
		saved, err := journal.claim(context.Background(), b, a)
		if lose {
			if !errors.Is(err, ErrUncertain) || saved != nil {
				t.Fatal(saved, err)
			}
		} else if err != nil || saved == nil || saved.Document == nil || saved.Document.ActionID != a.ID {
			t.Fatal(saved, err)
		}
		if f.inputs != 1 {
			t.Fatal("journal repeated input")
		}
	}
}

func TestHostDocumentOperationIsClosed(t *testing.T) {
	text := ""
	valid := DocumentOperation{Kind: "insert_text", Document: "document", Snapshot: "snapshot", Text: &text}
	if valid.Validate() != nil {
		t.Fatal("valid edit rejected")
	}
	for _, change := range []func(*DocumentOperation){func(o *DocumentOperation) { o.Command = "command" }, func(o *DocumentOperation) { o.Range = &DocumentTextRange{} }, func(o *DocumentOperation) { o.Snapshot = "" }, func(o *DocumentOperation) { o.Kind = "script" }, func(o *DocumentOperation) { o.Text = nil }} {
		op := valid
		change(&op)
		if op.Validate() == nil {
			t.Fatal("mixed operation accepted")
		}
	}
	op := Operation{Kind: "document", Document: &valid, Element: "unrelated"}
	if op.Validate() == nil {
		t.Fatal("mixed legacy/document operation accepted")
	}
}

func TestDocumentSupervisorRejectsInvalidEvidenceWithoutRedispatch(t *testing.T) {
	for _, change := range []func(*ControlResult){
		func(r *ControlResult) { r.Document = nil },
		func(r *ControlResult) { r.Document.ActionID = "foreign" },
		func(r *ControlResult) { r.Document.After.Target.ID = "another-window" },
		func(r *ControlResult) { r.Document.InputSHA256 = DocumentTextDigest("different input") },
		func(r *ControlResult) { r.Document.BeforeSnapshot = "different-snapshot" },
		func(r *ControlResult) { r.Outcome = "dispatched" },
		func(r *ControlResult) { r.Uncertain = true },
	} {
		e, f, binding, action := documentDispatchSetup(t, "insert_text")
		journal, err := OpenNativeJournal(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		s := NewNativeSupervisor(journal, LocalConsent{ID: "consent", Binding: binding, ApprovalMode: ApprovalFullTask, IssuedAt: now, ExpiresAt: now.Add(time.Minute), Active: true}, func(ctx context.Context, a PreparedControlAction) (ControlResult, error) {
			r, err := e.Dispatch(ctx, a)
			if err != nil {
				return r, err
			}
			change(&r)
			return r, nil
		})
		step := BoundedStep{ID: "step", Binding: binding, Actions: []PreparedControlAction{action}}
		for attempt := 0; attempt < 2; attempt++ {
			grant, err := s.Approve(step, func(BoundedStep) bool { return true })
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Execute(step, grant.ID)
			if !errors.Is(err, ErrUncertain) || f.inputs != 1 {
				t.Fatal("invalid evidence acknowledged or input retried", err, f.inputs)
			}
		}
		s.Stop()
		journal.Close()
	}
}

func TestDocumentJournalRevalidatesStoredEvidence(t *testing.T) {
	e, _, binding, action := documentDispatchSetup(t, "insert_text")
	j, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err = j.claim(context.Background(), binding, action); err != nil {
		t.Fatal(err)
	}
	r, err := e.Dispatch(context.Background(), action)
	if err != nil {
		t.Fatal(err)
	}
	if err = j.complete(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	// Simulate a damaged/restored row, not a new dispatch opportunity.
	if _, err = j.db.Exec("UPDATE native_actions SET result=? WHERE id=?", `{"action_id":"`+action.ID+`","outcome":"verified"}`, action.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = j.claim(context.Background(), binding, action); err == nil {
		t.Fatal("missing stored edit evidence accepted")
	}
}

func TestDocumentNewPreparationReobservesModelLatencyWithoutRenewingChangedState(t *testing.T) {
	for _, change := range []bool{false, true} {
		e, f, b, a := documentDispatchSetup(t, "insert_text")
		before, snapshot := cloneDocumentInventory(f.inventory), copyDocumentSnapshot(f.snapshot)
		before.Observation.CapturedAt = time.Now().Add(-time.Minute)
		if change {
			f.snapshot.Text = "user input"
			f.inventory.Active.Revision = "changed"
		}
		prepared, err := e.PrepareCurrent(context.Background(), b, *a.Operation.Document, before, snapshot)
		if change {
			if err == nil || f.inputs != 0 {
				t.Fatal("changed state refreshed", err)
			}
			continue
		}
		if err != nil || prepared.ID == a.ID || f.reads != 1 || f.inputs != 0 {
			t.Fatal(prepared, err)
		}
		r, err := e.Dispatch(context.Background(), prepared)
		if err != nil || r.Outcome != "verified" || f.inputs != 1 || r.Document.BeforeSnapshot != snapshot.ID || r.Document.Before.Observation == snapshot.Binding.Observation {
			t.Fatal("fresh preparation did not bind real read", r, err)
		}
	}
}

func TestDocumentCreationAfterApprovalDelayUsesFreshPreDispatchEvidence(t *testing.T) {
	e, f, _, a := documentDispatchSetup(t, "create_document")
	p := e.prepared[a.ID]
	p.inventory.Observation.CapturedAt = time.Now().Add(-time.Minute)
	e.prepared[a.ID] = p
	r, err := e.Dispatch(context.Background(), a)
	if err != nil || r.Outcome != "verified" || f.inputs != 1 {
		t.Fatal("unchanged delayed creation lost proof", r, err)
	}
}
