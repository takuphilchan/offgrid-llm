package computer

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func discoveryView(driver string) NativeView {
	v := NativeView{Observation: ControlObservation{ID: "obs", Target: TargetIdentity{ID: "target", OSSession: "os", ProcessGeneration: "generation", Surface: "window", Driver: driver}, Sequence: 1, StateDigest: strings.Repeat("a", 64), CapturedAt: time.Now().UTC()}}
	for i := 0; i < 400; i++ {
		v.Elements = append(v.Elements, NativeElement{ID: fmt.Sprintf("el-%d", i), Name: fmt.Sprintf("control %d", i), Role: "button"})
	}
	return v
}
func TestNativeDiscoveryPortableBoundsAndBinding(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		t.Run(driver, func(t *testing.T) {
			v := discoveryView(driver)
			v.Elements[399].Name = "Σίσυφος 日本語"
			before, _ := json.Marshal(v)
			raw, err := FindNativeControls(v, ControlQuery{Observation: "obs", Name: "ςίσΥΦΟΣ 日"}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			var p ControlPage
			_ = json.Unmarshal([]byte(raw), &p)
			if len(p.Controls) != 1 || p.Controls[0].ID != "el-399" {
				t.Fatal(raw)
			}
			q := ControlQuery{Observation: "obs"}
			seen := 0
			for {
				raw, err = FindNativeControls(v, q, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) > 4096 {
					t.Fatal("bytes")
				}
				p = ControlPage{}
				_ = json.Unmarshal([]byte(raw), &p)
				if len(p.Controls) > 16 {
					t.Fatal("rows")
				}
				seen += len(p.Controls)
				if p.Next == "" {
					break
				}
				q.Cursor = p.Next
			}
			if seen != 400 {
				t.Fatal("missing controls", seen)
			}
			q.Name = "other"
			if _, err = FindNativeControls(v, q, time.Now()); err == nil {
				t.Fatal("query-unbound cursor")
			}
			after, _ := json.Marshal(v)
			if string(before) != string(after) {
				t.Fatal("source mutated")
			}
			v.Limited = true
			raw, err = FindNativeControls(v, ControlQuery{Observation: "obs", Name: "missing"}, time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			p = ControlPage{}
			_ = json.Unmarshal([]byte(raw), &p)
			if !p.SourceLimited || p.InputFresh || p.Matches != 0 {
				t.Fatal(raw)
			}
		})
	}
}
func TestControlInspectionUnicodeAndSourceLimits(t *testing.T) {
	v := discoveryView("windows-uia")
	text := strings.Repeat("日\"<>&😊", 500)
	v.Elements[0].Name = strings.Repeat("long", 2000)
	v.Elements[0].Text = &text
	v.Elements[0].TextLimited = true
	q := ControlInspection{Observation: "obs", Element: "el-0"}
	joined := ""
	for {
		raw, err := InspectNativeControl(v, q, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > 4096 || !utf8.ValidString(raw) {
			t.Fatal("bounds")
		}
		var p ControlPage
		_ = json.Unmarshal([]byte(raw), &p)
		joined += *p.Controls[0].Text
		if !p.SourceLimited || !p.Controls[0].NameLimited {
			t.Fatal("lost limits")
		}
		if p.Next == "" {
			break
		}
		q.Cursor = p.Next
	}
	if joined != text {
		t.Fatal("corrupt Unicode pages")
	}
	v.Elements[0].ID = strings.Repeat("x", 5000)
	q = ControlInspection{Observation: "obs", Element: "el-0"}
	if _, err := InspectNativeControl(v, q, time.Now()); err == nil {
		t.Fatal("invalid identity")
	}
	if _, err := FindNativeControls(v, ControlQuery{Observation: "obs", Name: strings.Repeat("日", 90)}, time.Now()); err == nil {
		t.Fatal("oversized query")
	}
}
func TestEvidenceIdentityContinuityAndReplacement(t *testing.T) {
	first := CarryControlIdentities(nil, []string{"os-a", "os-b", "duplicate", "duplicate", ""})
	if first[0] == "" || first[2] != "" || first[3] != "" || first[4] != "" {
		t.Fatal(first)
	}
	next := CarryControlIdentities(map[string]string{"os-a": first[0], "os-b": first[1]}, []string{"os-a", "replacement"})
	if next[0] != first[0] || next[1] == first[1] {
		t.Fatal(next)
	}
	reset := CarryControlIdentities(nil, []string{"os-a"})
	if reset[0] == first[0] {
		t.Fatal("target change reused identity")
	}
}

func TestDiscoveryLeavesRoomForDurableStepMetadata(t *testing.T) {
	v := discoveryView("windows-uia")
	text := strings.Repeat("<&\"", 2000)
	for i := range v.Elements {
		v.Elements[i].Name = strings.Repeat("long name ", 40)
		v.Elements[i].Text = &text
	}
	find, err := FindNativeControls(v, ControlQuery{Observation: "obs"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	inspect, err := InspectNativeControl(v, ControlInspection{Observation: "obs", Element: "el-0"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []string{find, inspect} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal([]byte(result), &object); err != nil {
			t.Fatal(err)
		}
		object["offgrid_step_id"] = json.RawMessage("9223372036854775807")
		encoded, err := json.Marshal(object)
		if err != nil || len(encoded) > NativeDiscoveryMaxBytes {
			t.Fatalf("model-facing reply exceeds limit: %d bytes, %v", len(encoded), err)
		}
	}
}
