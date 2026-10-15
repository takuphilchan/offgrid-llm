package computer

import "testing"

func winUIDocumentFixture() []documentNode {
	return []documentNode{
		{key: "window"},
		{key: "tabs", parent: "window", className: "Microsoft.UI.Xaml.Controls.TabView"},
		{key: "tab-a", parent: "tabs", group: "tabs", className: "Microsoft.UI.Xaml.Controls.TabViewItem", tab: true, selected: true, selectable: true},
		{key: "tab-b", parent: "tabs", group: "tabs", className: "Microsoft.UI.Xaml.Controls.TabViewItem", tab: true, selectable: true},
		{key: "content", parent: "tabs", automationID: "TabContentPresenter", visible: true},
		{key: "editor", parent: "content", text: true, visible: true},
		{key: "new", parent: "tabs", automationID: "AddButton", visible: true, invokable: true},
	}
}

func TestWinUIDocumentProfileRequiresPackageRevisionAndExactStructure(t *testing.T) {
	if !notepadDocumentProfile("Microsoft.WindowsNotepad_8wekyb3d8bbwe!App", "Microsoft.WindowsNotepad_11.2607.14.0_x64__8wekyb3d8bbwe") {
		t.Fatal("reviewed profile absent")
	}
	for _, identity := range [][2]string{{"fake!App", "Microsoft.WindowsNotepad_11.2607.14.0_x64__8wekyb3d8bbwe"}, {"Microsoft.WindowsNotepad_8wekyb3d8bbwe!Other", "Microsoft.WindowsNotepad_11.2607.14.0_x64__8wekyb3d8bbwe"}, {"Microsoft.WindowsNotepad_8wekyb3d8bbwe!App", "Microsoft.WindowsNotepad_99.0.0.0_x64__8wekyb3d8bbwe"}} {
		if notepadDocumentProfile(identity[0], identity[1]) {
			t.Fatal("unknown app/revision accepted")
		}
	}
	nodes, commands, err := resolveWinUIDocumentProfile(winUIDocumentFixture(), "revision-a")
	if err != nil || len(commands) != 1 {
		t.Fatal(commands, err)
	}
	topology, err := resolveDocumentTopology(nodes, true)
	if err != nil || topology.editor != "editor" || topology.active != "tab-a" {
		t.Fatal(topology, err)
	}
	_, changed, _ := resolveWinUIDocumentProfile(winUIDocumentFixture(), "revision-b")
	if commands[0].ID == changed[0].ID {
		t.Fatal("package update retained command")
	}
	for _, change := range []func([]documentNode) []documentNode{
		func(n []documentNode) []documentNode { n[4].automationID = "New document"; return n },
		func(n []documentNode) []documentNode { n[4].protected = true; return n },
		func(n []documentNode) []documentNode { n[2].className = "UnrelatedTab"; return n },
		func(n []documentNode) []documentNode { n[3].selected = true; return n },
		func(n []documentNode) []documentNode { n[5].parent = "window"; return n },
		func(n []documentNode) []documentNode {
			return append(n, documentNode{key: "other", parent: "tabs", automationID: "AddButton", visible: true, invokable: true})
		},
	} {
		if _, _, err := resolveWinUIDocumentProfile(change(winUIDocumentFixture()), "revision"); err == nil {
			t.Fatal("ambiguous/changed layout accepted")
		}
	}
}
