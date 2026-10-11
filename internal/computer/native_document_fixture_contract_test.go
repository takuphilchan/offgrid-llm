package computer

import (
	"strconv"
	"strings"
	"testing"
)

// Private fixture protocol, deliberately not a service or companion API.
type unixDocumentFixtureReply struct {
	Version       int               `json:"version"`
	Kind          string            `json:"kind"`
	PID           int               `json:"pid"`
	Editor        string            `json:"editor"`
	SelectionUnit string            `json:"selection_unit"`
	Selected      int               `json:"selected"`
	Documents     []fixtureDocument `json:"documents"`
}

type fixtureDocument struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func validateUnixDocumentFixture(reply unixDocumentFixtureReply, pid int, target TargetIdentity) error {
	// Target comes from the native provider, not the fixture reply. The owned
	// child remains unreaped until cleanup, so its PID cannot be recycled.
	if pid <= 0 || reply.PID != pid || reply.Version != 1 || target.Validate() != nil ||
		!strings.HasPrefix(target.ProcessGeneration, strconv.Itoa(pid)+":") ||
		(target.Driver != "linux-atspi" && target.Driver != "macos-accessibility") ||
		reply.Editor != "shared-editor" || len(reply.Documents) < 2 || len(reply.Documents) > 16 ||
		reply.Selected < 0 || reply.Selected >= len(reply.Documents) {
		return ErrControlScope
	}
	unit := TextUnitScalar
	if target.Driver == "macos-accessibility" {
		unit = TextUnitUTF16
	}
	if (unit == TextUnitScalar && reply.SelectionUnit != "scalar") || (unit == TextUnitUTF16 && reply.SelectionUnit != "utf16") {
		return ErrControlScope
	}
	seen := map[int]bool{}
	for _, doc := range reply.Documents {
		if doc.ID <= 0 || seen[doc.ID] || ValidateDocumentInput(doc.Text) != nil {
			return ErrControlScope
		}
		seen[doc.ID] = true
		if _, err := DocumentRangeFromNative(doc.Text, DocumentTextRange{Start: doc.Start, End: doc.End}, unit); err != nil {
			return ErrControlScope
		}
	}
	return nil
}

func TestUnixDocumentFixtureRefusesUnownedOrMalformedState(t *testing.T) {
	target := TargetIdentity{ID: "target-1", OSSession: "session-1", ProcessGeneration: "123:456", Surface: "window-1", Driver: "linux-atspi"}
	valid := func() unixDocumentFixtureReply {
		return unixDocumentFixtureReply{Version: 1, Kind: "ready", PID: 123, Editor: "shared-editor", SelectionUnit: "scalar", Selected: 0, Documents: []fixtureDocument{{ID: 1, Text: "first"}, {ID: 2, Text: "other"}}}
	}
	if err := validateUnixDocumentFixture(valid(), 123, target); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*unixDocumentFixtureReply){
		func(v *unixDocumentFixtureReply) { v.PID = 124 }, func(v *unixDocumentFixtureReply) { v.Version = 2 },
		func(v *unixDocumentFixtureReply) { v.Editor = "foreign" }, func(v *unixDocumentFixtureReply) { v.Selected = 2 },
		func(v *unixDocumentFixtureReply) { v.SelectionUnit = "utf16" }, func(v *unixDocumentFixtureReply) { v.Documents[1].ID = 1 },
		func(v *unixDocumentFixtureReply) { v.Documents[0].End = 10 }, func(v *unixDocumentFixtureReply) { v.Documents[0].Text = "e\u0301"; v.Documents[0].End = 1 },
	} {
		reply := valid()
		change(&reply)
		if validateUnixDocumentFixture(reply, 123, target) == nil {
			t.Fatal("invalid fixture accepted")
		}
	}
	for _, generation := range []string{"12:456", "1234:456", "124:456", ""} {
		other := target
		other.ProcessGeneration = generation
		if validateUnixDocumentFixture(valid(), 123, other) == nil {
			t.Fatal("foreign target accepted")
		}
	}
	target.Driver = "macos-accessibility"
	reply := valid()
	reply.SelectionUnit = "utf16"
	reply.Documents[0] = fixtureDocument{ID: 1, Text: "😀", End: 2}
	if err := validateUnixDocumentFixture(reply, 123, target); err != nil {
		t.Fatal(err)
	}
	reply.Documents[0].End = 1
	if validateUnixDocumentFixture(reply, 123, target) == nil {
		t.Fatal("split surrogate accepted")
	}
}
