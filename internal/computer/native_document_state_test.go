package computer

import "testing"

func TestNativeDocumentStateDoesNotMergeDuplicateTitlesOrReusedEditors(t *testing.T) {
	target := TargetIdentity{ID: "window", OSSession: "session", ProcessGeneration: "generation", Surface: "surface", Driver: "linux-atspi"}
	top := documentTopology{tabs: []documentNode{{key: "one", title: "Untitled", selected: true, selectable: true}, {key: "two", title: "Untitled", selectable: true}}, active: "one", editor: "shared-editor"}
	r := &DocumentTextRange{}
	var state *nativeDocumentState
	first, _, state, err := state.inventory(target, top, "", r, "stable", DocumentCapabilities{Read: true}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	top.tabs[0].selected = false
	top.tabs[1].selected = true
	top.active = "two"
	second, _, state, err := state.inventory(target, top, "", r, "stable", DocumentCapabilities{Read: true}, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Active.Document == second.Active.Document || first.Active.Editor != second.Active.Editor {
		t.Fatal("reused editor merged documents")
	}
	top.tabs[0], top.tabs[1] = top.tabs[1], top.tabs[0]
	third, _, state, err := state.inventory(target, top, "changed", r, "stable", DocumentCapabilities{Read: true}, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if third.Active.Document != second.Active.Document || third.Active.Generation != second.Active.Generation || third.Active.Revision == second.Active.Revision {
		t.Fatal("reordering/text changed identity instead of revision")
	}
	top.editor = "replaced-editor"
	fourth, _, _, err := state.inventory(target, top, "changed", r, "stable", DocumentCapabilities{Read: true}, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if fourth.Active.Generation == third.Active.Generation {
		t.Fatal("replacement retained generation")
	}
}
