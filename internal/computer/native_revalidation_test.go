package computer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMatchingReobservationRequiresUnchangedCompleteState(t *testing.T) {
	step, _, _, _ := controlFixture()
	now := time.Now()
	text, checked := "Unicode: العربية 中文", false
	old := NativeView{Observation: ControlObservation{ID: "old-view", Target: step.Binding.Target, CapturedAt: now.Add(-3 * time.Minute), Sequence: 1, StateDigest: strings.Repeat("a", 64)}, Elements: []NativeElement{{ID: "old-edit", Name: "Query", Role: "edit", Writable: true, Text: &text}, {ID: "old-check", Name: "Option", Role: "checkbox", Checkable: true, Checked: &checked}}}
	fresh := old
	fresh.Observation.ID, fresh.Observation.CapturedAt, fresh.Observation.Sequence = "fresh-view", now, 2
	fresh.Elements = append([]NativeElement(nil), old.Elements...)
	fresh.Elements[0].ID, fresh.Elements[1].ID = "new-edit", "new-check"
	if old.Observation.Validate(step.Binding.Target, now) != ErrStaleObservation {
		t.Fatal("normal freshness check was weakened")
	}
	ids, err := matchingReobservation(old, fresh, now)
	if err != nil || ids["new-edit"] != "old-edit" || ids["new-check"] != "old-check" || fresh.Elements[0].ID != "new-edit" {
		t.Fatal("lossless mapping", ids, err)
	}
	for name, change := range map[string]func(*NativeView){
		"runtime/state changed": func(v *NativeView) { v.Observation.StateDigest = strings.Repeat("b", 64) },
		"process changed":       func(v *NativeView) { v.Observation.Target.ProcessGeneration = "other" },
		"old identity":          func(v *NativeView) { v.Observation.ID = old.Observation.ID },
		"old sequence":          func(v *NativeView) { v.Observation.Sequence = old.Observation.Sequence },
		"stale recapture":       func(v *NativeView) { v.Observation.CapturedAt = now.Add(-time.Minute) },
		"future capture":        func(v *NativeView) { v.Observation.CapturedAt = now.Add(time.Second) },
		"incomplete":            func(v *NativeView) { v.Limited = true },
		"removed":               func(v *NativeView) { v.Elements = v.Elements[:1] },
		"name changed":          func(v *NativeView) { v.Elements[0].Name = "Password" },
		"role changed":          func(v *NativeView) { v.Elements[0].Role = "button" },
		"value changed":         func(v *NativeView) { x := "changed"; v.Elements[0].Text = &x },
		"checked changed":       func(v *NativeView) { x := true; v.Elements[1].Checked = &x },
		"capability changed":    func(v *NativeView) { v.Elements[0].Writable = false },
		"truncation changed":    func(v *NativeView) { v.Elements[0].TextLimited = true },
		"duplicate identity":    func(v *NativeView) { v.Elements[1].ID = v.Elements[0].ID },
		"invalid identity":      func(v *NativeView) { v.Elements[0].ID = "" },
		"order changed":         func(v *NativeView) { v.Elements[0], v.Elements[1] = v.Elements[1], v.Elements[0] },
	} {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(fresh)
			var changed NativeView
			_ = json.Unmarshal(data, &changed)
			change(&changed)
			if _, err := matchingReobservation(old, changed, now); err != ErrStaleObservation {
				t.Fatal("changed or invalid observation accepted", err)
			}
		})
	}
	old.Limited = true
	if _, err := matchingReobservation(old, fresh, now); err != ErrStaleObservation {
		t.Fatal("incomplete original view accepted")
	}
}
