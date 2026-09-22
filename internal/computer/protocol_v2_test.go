package computer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func controlFixture() (BoundedStep, StepGrant, LocalConsent, time.Time) {
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	text := "Zimbabwe research — final (2026)"
	binding := ControlBinding{Actor: "alice", Task: "task", Companion: "host", Session: "session", Target: TargetIdentity{ID: "target", OSSession: "os", ProcessGeneration: "generation", Surface: "window", Driver: "windows-uia"}}
	step := BoundedStep{ID: "step", Binding: binding, Actions: []PreparedControlAction{{ID: "action", Operation: Operation{Kind: "replace_text", Element: "element", Text: &text}, ControlIdentity: "stable-element", Precondition: strings.Repeat("a", 64), ExpectedChange: strings.Repeat("b", 64), ApprovalClass: ActionReversible}}}
	digest, _ := step.Digest()
	consent := LocalConsent{ID: "consent", Binding: binding, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(10 * time.Minute), Active: true}
	grant := StepGrant{ID: "approval", ConsentID: consent.ID, Binding: binding, StepDigest: digest, IssuedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	return step, grant, consent, now
}

func TestBoundedStepBindsExactActionsAndCurrentLocalConsent(t *testing.T) {
	step, grant, consent, now := controlFixture()
	if err := grant.Validate(step, consent, now); err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*BoundedStep, *StepGrant, *LocalConsent){
		"actor": func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) { s.Binding.Actor = "bob" },
		"generation": func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) {
			s.Binding.Target.ProcessGeneration = "replacement"
		},
		"element": func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) { s.Actions[0].Operation.Element = "other" },
		"value": func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) {
			text := "different"
			s.Actions[0].Operation.Text = &text
		},
		"precondition": func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) {
			s.Actions[0].Precondition = strings.Repeat("c", 64)
		},
		"target":          func(s *BoundedStep, _ *StepGrant, _ *LocalConsent) { s.Binding.Target.ID = "other" },
		"restart":         func(_ *BoundedStep, _ *StepGrant, c *LocalConsent) { c.ID = "new-consent" },
		"revoked":         func(_ *BoundedStep, _ *StepGrant, c *LocalConsent) { c.Active = false },
		"expired":         func(_ *BoundedStep, g *StepGrant, _ *LocalConsent) { g.ExpiresAt = now },
		"long expiry":     func(_ *BoundedStep, g *StepGrant, _ *LocalConsent) { g.ExpiresAt = now.Add(6 * time.Minute) },
		"session expiry":  func(_ *BoundedStep, _ *StepGrant, c *LocalConsent) { c.ExpiresAt = now.Add(time.Minute) },
		"future approval": func(_ *BoundedStep, g *StepGrant, _ *LocalConsent) { g.IssuedAt = now.Add(time.Second) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s, g, c, _ := controlFixture()
			change(&s, &g, &c)
			if err := g.Validate(s, c, now); err == nil {
				t.Fatal("changed authority accepted")
			}
		})
	}
}

func TestBoundedStepLimitsAndConsequentialActionIsolation(t *testing.T) {
	step, _, _, _ := controlFixture()
	action := step.Actions[0]
	for i := 1; i < MaxStepActions; i++ {
		next := action
		next.ID = strings.Repeat("a", i)
		step.Actions = append(step.Actions, next)
	}
	if _, err := step.Digest(); err != nil {
		t.Fatal(err)
	}
	step.Actions = append(step.Actions, action)
	if _, err := step.Digest(); err == nil {
		t.Fatal("eleven-action plan accepted")
	}
	step.Actions = step.Actions[:2]
	step.Actions[1].Operation = Operation{Kind: "activate", Element: "send-button"}
	if _, err := step.Digest(); !errors.Is(err, ErrControlApproval) {
		t.Fatalf("unknown click batched with edits: %v", err)
	}
	step.Actions = step.Actions[1:]
	if _, err := step.Digest(); err != nil {
		t.Fatal(err)
	}
}

func TestControlOperationsRejectCodeUnknownFieldsAndTraversal(t *testing.T) {
	for _, data := range []string{
		`{"kind":"observe","text":"smuggled"}`,
		`{"kind":"replace_text","element":"el","text":"ok","approved":true}`,
		`{"kind":"shell","text":"anything"}`,
		`{"kind":"shortcut","shortcut":"Run PowerShell"}`,
		`{"kind":"navigate","destination":"file:///private"}`,
		`{"kind":"trash_file","file":{"folder_grant":"scope","source":"../private"}}`,
		`{"kind":"create_file","file":{"folder_grant":"scope","name":"../outside"}}`,
		`{"kind":"permanent_delete","file":{"folder_grant":"scope","source":"file"}}`,
		`{"kind":"observe"} {"kind":"activate"}`,
		`null`,
	} {
		var op Operation
		if err := DecodeControlMessage([]byte(data), &op); err == nil && op.Validate() == nil {
			t.Errorf("accepted %s", data)
		}
	}
	op := Operation{Kind: "set_checked", Element: "el"}
	if op.Validate() == nil {
		t.Fatal("missing bool accepted")
	}
	value := false
	op.Checked = &value
	if err := op.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(op)
	if string(encoded) != `{"kind":"set_checked","element":"el","checked":false}` {
		t.Fatalf("checked action changed shape: %s", encoded)
	}
	if err := (Operation{Kind: "shortcut", Element: "field", Shortcut: "paste"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Operation{Kind: "shortcut", Shortcut: "save"}).Validate(); err == nil {
		t.Fatal("unbound shortcut accepted")
	}
}

func TestObservationRejectsStaleFutureAndReplacedTargets(t *testing.T) {
	step, _, _, now := controlFixture()
	obs := ControlObservation{ID: "observation", Target: step.Binding.Target, Sequence: 1, CapturedAt: now, StateDigest: strings.Repeat("a", 64)}
	if err := obs.Validate(step.Binding.Target, now); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(obs.Validate(step.Binding.Target, now.Add(16*time.Second)), ErrStaleObservation) {
		t.Fatal("old observation accepted")
	}
	if !errors.Is(obs.Validate(step.Binding.Target, now.Add(-time.Second)), ErrStaleObservation) {
		t.Fatal("future observation accepted")
	}
	obs.Target.ProcessGeneration = "replacement"
	if !errors.Is(obs.Validate(step.Binding.Target, now), ErrControlScope) {
		t.Fatal("replaced process accepted")
	}
}

func TestPrivateWorkerFramingFailsClosed(t *testing.T) {
	var buffer bytes.Buffer
	want := Operation{Kind: "observe"}
	if err := WriteControlFrame(&buffer, want); err != nil {
		t.Fatal(err)
	}
	encoded := append([]byte{}, buffer.Bytes()...)
	var got Operation
	if err := ReadControlFrame(&buffer, &got); err != nil || got.Kind != want.Kind {
		t.Fatalf("frame: %+v %v", got, err)
	}
	if !errors.Is(ReadControlFrame(bytes.NewReader(encoded[:len(encoded)-1]), &got), io.ErrUnexpectedEOF) {
		t.Fatal("truncated frame accepted")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], MaxIPCFrame+1)
	if !errors.Is(ReadControlFrame(bytes.NewReader(header[:]), &got), ErrInvalidControl) {
		t.Fatal("oversized frame accepted")
	}
}

func FuzzControlOperationDecode(f *testing.F) {
	f.Add([]byte(`{"kind":"observe"}`))
	f.Add([]byte(`{"kind":"replace_text","element":"el","text":"hello"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var op Operation
		if DecodeControlMessage(data, &op) != nil || op.Validate() != nil {
			return
		}
		encoded, err := json.Marshal(op)
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip Operation
		if DecodeControlMessage(encoded, &roundtrip) != nil || roundtrip.Validate() != nil {
			t.Fatal("accepted operation cannot roundtrip")
		}
	})
}
