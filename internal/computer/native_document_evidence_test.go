package computer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDocumentReadbackRequiresCompleteNativeSnapshot(t *testing.T) {
	_, snapshot := documentFixture()
	snapshot.Text = "café 日本語 😀"
	snapshot.Selection = nil
	proof, err := NewDocumentReadback(snapshot)
	if err != nil || proof.TextSHA256 != DocumentTextDigest(snapshot.Text) || proof.Bytes != len(snapshot.Text) {
		t.Fatal(proof, err)
	}
	data, _ := json.Marshal(proof)
	if strings.Contains(string(data), snapshot.Text) {
		t.Fatal("private text in evidence projection")
	}
	for _, change := range []func(*DocumentSnapshot){
		func(s *DocumentSnapshot) { s.Complete = false }, func(s *DocumentSnapshot) { s.Composition = "unknown" },
		func(s *DocumentSnapshot) { s.Binding.Generation = "" }, func(s *DocumentSnapshot) { s.Text = "\x00" },
	} {
		copy := snapshot
		change(&copy)
		if _, err := NewDocumentReadback(copy); err == nil {
			t.Fatal("invalid snapshot certified")
		}
	}
	proof.Bytes = 0
	if proof.Validate() == nil {
		t.Fatal("nonempty digest certified as blank")
	}
}

func TestDocumentReadbackRejectsMalformedPageAndSelection(t *testing.T) {
	_, snapshot := documentFixture()
	snapshot.Text = "abc"
	proof, err := NewDocumentReadback(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	page := DocumentTextPage{Snapshot: snapshot.ID, Binding: snapshot.Binding, Text: "abc", End: 3, TotalScalars: 3, SourceComplete: true}
	proof.Page = &page
	if err := proof.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*DocumentReadback){
		func(r *DocumentReadback) { r.Page.Text = "a\x00c"; r.TextSHA256 = DocumentTextDigest(r.Page.Text) },
		func(r *DocumentReadback) { r.Page.Binding.Revision = "different" },
		func(r *DocumentReadback) { r.Page.Text = "abd" },
		func(r *DocumentReadback) { r.Selection = &DocumentTextRange{Start: 1, End: 4} },
	} {
		candidate, part := proof, page
		candidate.Page = &part
		mutate(&candidate)
		if candidate.Validate() == nil {
			t.Fatal("invalid page accepted")
		}
	}
}

func TestDocumentEditEvidencePinsPreparedPreimageAndActualPostimage(t *testing.T) {
	beforeView, before, op := textOperationFixture("replace_selection", "日本語", DocumentTextRange{Start: 2, End: 6})
	plan, err := PrepareDocumentText(beforeView, before, op, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.ID = "post-snapshot"
	after.Text = plan.ExpectedText()
	after.Binding.Observation = "post-observation"
	after.Binding.Revision = "post-revision"
	selection := plan.ExpectedSelection()
	after.Selection = &selection
	view := beforeView
	view.Observation.ID = after.Binding.Observation
	binding := after.Binding
	view.Active = &binding
	proof, err := NewDocumentEditEvidence(plan, view, after, time.Now())
	if err != nil || proof.ActionID != plan.ID() || proof.ExpectedSHA256 != DocumentTextDigest(plan.ExpectedText()) || proof.PreimageSHA256 != DocumentTextDigest(before.Text) || proof.InputSHA256 != DocumentTextDigest(*op.Text) {
		t.Fatal(proof, err)
	}
	for _, change := range []func(*DocumentSnapshot){
		func(s *DocumentSnapshot) { s.Text = *op.Text }, func(s *DocumentSnapshot) { s.Text = "prefix changed" },
		func(s *DocumentSnapshot) { s.Binding.Document = "another-tab" }, func(s *DocumentSnapshot) { s.Binding.Generation = "replaced" },
		func(s *DocumentSnapshot) { s.Binding.Container = "another-container" }, func(s *DocumentSnapshot) { s.Binding.Editor = "replaced-editor" },
		func(s *DocumentSnapshot) { s.ID = before.ID }, func(s *DocumentSnapshot) { s.Binding.Observation = before.Binding.Observation },
		func(s *DocumentSnapshot) { s.Composition = "active" }, func(s *DocumentSnapshot) { s.Complete = false; s.Selection = nil },
	} {
		candidate := after
		change(&candidate)
		if _, err := NewDocumentEditEvidence(plan, view, candidate, time.Now()); err == nil {
			t.Fatal("mismatched read-back certified")
		}
	}
	if _, err := NewDocumentEditEvidence(PreparedDocumentText{}, view, after, time.Now()); err == nil {
		t.Fatal("missing preparation accepted")
	}
}

func TestDocumentCreationEvidenceRequiresObservedCommandAndNewTab(t *testing.T) {
	before, _ := documentFixture()
	after, snapshot := documentFixture()
	after.Observation.ID = "created"
	after.Active.Observation = "created"
	after.Active.Document = "new"
	after.Active.Container = "new-tab"
	after.Active.Generation = "new-generation"
	doc := after.Documents[0]
	doc.ID, doc.Container, doc.Generation = after.Active.Document, after.Active.Container, after.Active.Generation
	after.Documents[0].Selected = false
	after.Documents = append(after.Documents, doc)
	snapshot.Binding = *after.Active
	snapshot.ID = "created-snapshot"
	proof, err := NewDocumentCreationEvidence("action", "create", before, after, snapshot, time.Now())
	if err != nil || proof.ExpectedSHA256 != DocumentTextDigest("") {
		t.Fatal(proof, err)
	}
	if _, err := NewDocumentCreationEvidence("action", "unobserved", before, after, snapshot, time.Now()); err == nil {
		t.Fatal("invented command certified")
	}
	snapshot.Text = "restored content"
	if _, err := NewDocumentCreationEvidence("action", "create", before, after, snapshot, time.Now()); err == nil {
		t.Fatal("restored document certified blank")
	}
}
