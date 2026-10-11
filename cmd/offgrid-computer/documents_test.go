package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// No native APIs or user applications are involved in these worker tests.
type workerDocumentFixture struct {
	computer.NativeAdapter
	view          computer.DocumentInventory
	snapshot      computer.DocumentSnapshot
	reads, writes int
}

func (f *workerDocumentFixture) DocumentContractVersion() int { return 1 }
func (f *workerDocumentFixture) ReadDocument(ctx context.Context, b computer.ControlBinding) (computer.DocumentInventory, computer.DocumentSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return computer.DocumentInventory{}, computer.DocumentSnapshot{}, err
	}
	if b.Target != f.view.Observation.Target {
		return computer.DocumentInventory{}, computer.DocumentSnapshot{}, computer.ErrControlScope
	}
	f.reads++
	f.view.Observation.ID = fmt.Sprintf("observation-%d", f.reads)
	f.view.Observation.CapturedAt = time.Now()
	f.snapshot.Binding.Observation = f.view.Observation.ID
	f.snapshot.ID = fmt.Sprintf("snapshot-%d", f.reads)
	binding := f.snapshot.Binding
	view := f.view
	view.Active = &binding
	snapshot := f.snapshot
	selection := *f.snapshot.Selection
	snapshot.Selection = &selection
	return view, snapshot, nil
}
func (f *workerDocumentFixture) ApplyDocumentText(ctx context.Context, b computer.ControlBinding, p computer.PreparedDocumentText) error {
	f.writes++
	f.snapshot.Text = p.ExpectedText()
	r := p.ExpectedSelection()
	f.snapshot.Selection = &r
	f.snapshot.Binding.Revision = fmt.Sprintf("revision-%d", f.writes)
	return nil
}
func (f *workerDocumentFixture) ApplyDocumentLifecycle(context.Context, computer.ControlBinding, computer.DocumentInventory, computer.DocumentOperation) error {
	return computer.ErrDocumentUnavailable
}
func newWorkerDocumentFixture() (*workerDocumentFixture, computer.ControlBinding) {
	target := computer.TargetIdentity{ID: "window", OSSession: "session", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	binding := computer.ControlBinding{Actor: "actor", Task: "task", Companion: "host", Session: "session", Target: target}
	doc := computer.DocumentBinding{Target: target, Document: "document", Generation: "generation", Container: "tab", Editor: "editor", Observation: "observation", Revision: "revision"}
	f := &workerDocumentFixture{
		view:     computer.DocumentInventory{Version: 1, Complete: true, Observation: computer.ControlObservation{ID: "observation", Target: target, Sequence: 1, CapturedAt: time.Now(), StateDigest: strings.Repeat("a", 64)}, Documents: []computer.NativeDocument{{ID: "document", Generation: "generation", Container: "tab", Selected: true, Capabilities: computer.DocumentCapabilities{Read: true, InsertText: true}}}, Active: &doc},
		snapshot: computer.DocumentSnapshot{ID: "snapshot", Binding: doc, Text: "", Complete: true, Selection: &computer.DocumentTextRange{}, Composition: "stable"},
	}
	return f, binding
}

func TestDocumentWorkerReadPrepareJournalAndExactReadback(t *testing.T) {
	f, b := newWorkerDocumentFixture()
	w := newDocumentWorker(f)
	view, err := w.documents(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	read, err := w.read(context.Background(), b, view.Observation)
	if err != nil || read.Binding.Observation != view.Observation.ID || read.Validate() != nil {
		t.Fatal(read, err)
	}
	text := "OffGrid test — café 日本語 12345"
	op := computer.Operation{Kind: "document", Document: &computer.DocumentOperation{Kind: "insert_text", Document: read.Binding.Document, Snapshot: read.Snapshot, Text: &text}}
	a, err := w.prepare(context.Background(), b, op, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := computer.OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	now := time.Now()
	s := computer.NewNativeSupervisor(journal, computer.LocalConsent{ID: "consent", Binding: b, ApprovalMode: computer.ApprovalFullTask, IssuedAt: now, ExpiresAt: now.Add(time.Minute), Active: true}, w.dispatch)
	defer s.Stop()
	step := computer.BoundedStep{ID: "step", Binding: b, Actions: []computer.PreparedControlAction{a}}
	for attempt := 0; attempt < 2; attempt++ {
		grant, err := s.Approve(step, func(computer.BoundedStep) bool { return true })
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.Execute(step, grant.ID)
		if err != nil || len(result) != 1 || result[0].Document == nil || result[0].Document.ExpectedSHA256 != computer.DocumentTextDigest(text) || f.writes != 1 {
			t.Fatal(result, err, f.writes)
		}
	}
	if _, err = w.prepare(context.Background(), b, op, view.Observation); err == nil {
		t.Fatal("consumed inventory remained usable")
	}
	s.Stop()
	if _, err = s.Approve(step, func(computer.BoundedStep) bool { return true }); err == nil {
		t.Fatal("stopped supervisor approved input")
	}
}

func TestDocumentWorkerRejectsChangedSelectionAndForeignScopeBeforeInput(t *testing.T) {
	for _, change := range []func(*workerDocumentFixture, *computer.ControlBinding, *computer.ControlObservation){
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			b.Actor = "other"
		},
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			b.Target.ProcessGeneration = "restarted"
		},
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			o.ID = "old"
		},
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			f.snapshot.Binding.Revision = "changed"
		},
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			f.snapshot.Text = "user typed"
			f.snapshot.Selection = &computer.DocumentTextRange{Start: 1, End: 1}
		},
		func(f *workerDocumentFixture, b *computer.ControlBinding, o *computer.ControlObservation) {
			f.snapshot.Composition = "active"
		},
	} {
		f, b := newWorkerDocumentFixture()
		w := newDocumentWorker(f)
		view, err := w.documents(context.Background(), b)
		if err != nil {
			t.Fatal(err)
		}
		change(f, &b, &view.Observation)
		if _, err = w.read(context.Background(), b, view.Observation); err == nil || f.writes != 0 {
			t.Fatal("stale/foreign read accepted", err)
		}
	}
}

func TestDocumentWorkerClosedProtocolAndUnavailableLegacy(t *testing.T) {
	var w *documentWorker
	w.clear()
	if _, err := w.documents(context.Background(), computer.ControlBinding{}); !errors.Is(err, computer.ErrDocumentUnavailable) {
		t.Fatal(err)
	}
	for _, protocol := range []int{2, 3} {
		if !(request{Protocol: protocol, ID: "r", Kind: "documents"}).valid() || !(request{Protocol: protocol, ID: "r", Kind: "read_document", Observation: &computer.ControlObservation{}}).valid() {
			t.Fatal("document read shape refused")
		}
		for _, req := range []request{{Protocol: protocol, ID: "r", Kind: "documents", Target: "other"}, {Protocol: protocol, ID: "r", Kind: "read_document"}, {Protocol: protocol, ID: "r", Kind: "read_document", Observation: &computer.ControlObservation{}, Grant: "not-authority"}} {
			if req.valid() {
				t.Fatal("mixed document request accepted")
			}
		}
	}
}

func TestDocumentWorkerPagesKeepOneSnapshotAndRefuseChangedRevision(t *testing.T) {
	f, b := newWorkerDocumentFixture()
	f.snapshot.Text = strings.Repeat("日本語😀", 1000)
	w := newDocumentWorker(f)
	v, err := w.documents(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.read(context.Background(), b, v.Observation)
	if err != nil {
		t.Fatal(err)
	}
	if r.Page == nil || r.Page.Next == nil || len(r.Page.Text) > computer.MaxDocumentPageBytes {
		t.Fatal("missing bounded first page")
	}
	snapshot := r.Snapshot
	actual := r.Page.Text
	for r.Page.Next != nil {
		query := &computer.DocumentPageRequest{Snapshot: snapshot, Revision: r.Binding.Revision, Start: *r.Page.Next}
		r, err = w.read(context.Background(), b, v.Observation, query)
		if err != nil || r.Snapshot != snapshot || r.Page.Snapshot != snapshot {
			t.Fatal("page identity drift", err)
		}
		actual += r.Page.Text
	}
	if actual != f.snapshot.Text {
		t.Fatal("pages changed exact text")
	}
	query := &computer.DocumentPageRequest{Snapshot: snapshot, Revision: r.Binding.Revision, Start: 0}
	f.snapshot.Binding.Revision = "user-edit"
	if _, err = w.read(context.Background(), b, v.Observation, query); err == nil {
		t.Fatal("mixed revisions in pages")
	}
	if f.writes != 0 {
		t.Fatal("pagination dispatched input")
	}
}
