package computer

import "testing"

func documentTreeFixture() []documentNode {
	return []documentNode{
		{key: "window"}, {key: "tabs", parent: "window"},
		{key: "a", parent: "tabs", group: "tabs", tab: true, selected: true, selectable: true, title: "Untitled", controls: []string{"pane"}},
		{key: "b", parent: "tabs", group: "tabs", tab: true, selectable: true, title: "Untitled"},
		{key: "pane", parent: "window"}, {key: "editor", parent: "pane", text: true, visible: true},
	}
}

func TestDocumentTopologyUsesRelationshipsNotTitleOrderOrEditorIdentity(t *testing.T) {
	nodes := documentTreeFixture()
	first, err := resolveDocumentTopology(nodes, true)
	if err != nil || len(first.tabs) != 2 || first.active != "a" || first.editor != "editor" {
		t.Fatal(first, err)
	}
	nodes[2], nodes[3] = nodes[3], nodes[2] // Reordered tabs and identical captions.
	nodes[2].selected, nodes[3].selected = true, false
	nodes[2].controls, nodes[3].controls = []string{"pane"}, nil
	second, err := resolveDocumentTopology(nodes, true)
	if err != nil || second.active != "b" || second.editor != first.editor {
		t.Fatal("reused editor erased document identity", second, err)
	}
	// A pane can instead be a real child of the tab. No ControllerFor needed.
	nodes[4].parent = "b"
	nodes[2].controls = nil
	if _, err := resolveDocumentTopology(nodes, true); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentTopologyRefusesAmbiguousOrIncompleteRelationships(t *testing.T) {
	for name, mutate := range map[string]func([]documentNode) []documentNode{
		"no relationship":        func(n []documentNode) []documentNode { n[2].controls = nil; return n },
		"both tabs claim editor": func(n []documentNode) []documentNode { n[3].controls = []string{"pane"}; return n },
		"both selected":          func(n []documentNode) []documentNode { n[3].selected = true; return n },
		"no selection":           func(n []documentNode) []documentNode { n[2].selected = false; return n },
		"foreign pane":           func(n []documentNode) []documentNode { n[2].controls = []string{"other-window"}; return n },
		"whole window link":      func(n []documentNode) []documentNode { n[2].controls = []string{"window"}; return n },
		"protected editor":       func(n []documentNode) []documentNode { n[5].protected = true; return n },
		"hidden editor":          func(n []documentNode) []documentNode { n[5].visible = false; return n },
		"two editors": func(n []documentNode) []documentNode {
			return append(n, documentNode{key: "second-editor", parent: "pane", text: true, visible: true})
		},
		"cycle":                       func(n []documentNode) []documentNode { n[4].parent = "editor"; return n },
		"missing parent":              func(n []documentNode) []documentNode { n[4].parent = "missing"; return n },
		"duplicate provider identity": func(n []documentNode) []documentNode { n[3].key = "a"; return n },
		"unknown group":               func(n []documentNode) []documentNode { n[3].group = "missing"; return n },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveDocumentTopology(mutate(documentTreeFixture()), true); err == nil {
				t.Fatal("ambiguous document became editable")
			}
		})
	}
	if _, err := resolveDocumentTopology(documentTreeFixture(), false); err == nil {
		t.Fatal("truncated enumeration passed")
	}
}

func TestDocumentTopologyReadsOuterDocumentNotNestedControls(t *testing.T) {
	nodes := append(documentTreeFixture(), documentNode{key: "paragraph", parent: "editor", text: true, visible: true})
	got, err := resolveDocumentTopology(nodes, true)
	if err != nil || got.editor != "editor" {
		t.Fatal(got, err)
	}
	nodes[len(nodes)-1].protected = true
	if _, err := resolveDocumentTopology(nodes, true); err == nil {
		t.Fatal("outer TextPattern bypassed protected descendant")
	}
}
