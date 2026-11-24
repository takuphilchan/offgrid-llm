package capabilities

import "testing"

func TestBrowserStructuredReadRequiresExactTrustedDescriptor(t *testing.T) {
	for _, name := range []string{"browser_search", "browser_frames", "browser_frame_select"} {
		d := Descriptor{Name: name, Namespace: "computer", Source: "local-companion", Kind: Computer, Risk: RiskLow, Revision: BrowserEffectsRevision}
		if !TrustedBrowserRead(d) || !TrustedBrowserOperation(d) {
			t.Fatal("structured read not recognized", name)
		}
		for _, mutate := range []func(*Descriptor){
			func(d *Descriptor) { d.Source = "mcp:local-companion" }, func(d *Descriptor) { d.Namespace = "tools" }, func(d *Descriptor) { d.Kind = Read }, func(d *Descriptor) { d.Risk = RiskHigh }, func(d *Descriptor) { d.Revision = "" }, func(d *Descriptor) { d.Name = "browser_unreviewed" },
		} {
			other := d
			mutate(&other)
			if TrustedBrowserRead(other) || TrustedBrowserOperation(other) {
				t.Fatal("unreviewed descriptor trusted", other)
			}
		}
	}
}
