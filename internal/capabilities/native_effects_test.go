package capabilities

import "testing"

func TestNativeEffectsRequireReviewedIdentityAndRevision(t *testing.T) {
	d := Descriptor{Name: "computer_observe", Namespace: "computer", Source: "local-companion", Kind: Computer, Risk: RiskLow, Revision: NativeEffectsRevision}
	if !TrustedNativeRead(d) {
		t.Fatal("reviewed native read rejected")
	}
	for _, change := range []func(*Descriptor){
		func(d *Descriptor) { d.Source = "mcp" },
		func(d *Descriptor) { d.Namespace = "tools" },
		func(d *Descriptor) { d.Kind = Read },
		func(d *Descriptor) { d.Revision = "" },
		func(d *Descriptor) { d.Revision = "native-effects-v2" },
		func(d *Descriptor) { d.Name = "computer_capture" },
		func(d *Descriptor) { d.Name = "computer_open_application" },
		func(d *Descriptor) { d.Risk = RiskHigh },
	} {
		copy := d
		change(&copy)
		if TrustedNativeRead(copy) {
			t.Fatalf("unreviewed read accepted: %+v", copy)
		}
	}
	for _, name := range []string{"browser_observe", "computer_prepare", "invented"} {
		if _, known := NativeOperationEffect(name, NativeEffectsRevision); known {
			t.Fatalf("operation silently classified: %s", name)
		}
	}
}
