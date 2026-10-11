package server

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func TestNativeDescriptorsUseSharedReviewedEffects(t *testing.T) {
	for _, tool := range append(nativeComputerTools(), desktopComputerTools()...) {
		name := tool.Function.Name
		d, exists := browserDescriptor(name)
		effect, known := capabilities.NativeOperationEffect(name, capabilities.NativeEffectsRevision)
		if !exists || !known || d.Revision != capabilities.NativeEffectsRevision {
			t.Fatalf("unversioned native descriptor: %+v", d)
		}
		if capabilities.TrustedNativeRead(d) != (effect == capabilities.NativeRead) {
			t.Fatalf("effect disagreement: %+v", d)
		}
	}
	for _, tool := range browserTools() {
		d, _ := browserDescriptor(tool.Function.Name)
		if d.Revision != "" || capabilities.TrustedNativeRead(d) {
			t.Fatal("legacy browser semantics changed")
		}
	}
}
