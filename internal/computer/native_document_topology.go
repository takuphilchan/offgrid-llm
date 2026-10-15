package computer

// Provider-private, locale-independent relationships. A tab's pane must either
// be its descendant or explicitly referenced by the accessibility provider.
// Names, child order, the focused control and text equality never make a link.
type documentNode struct {
	key, parent, group      string
	title                   string
	tab, selected           bool
	selectable              bool
	text, visible           bool
	protected               bool
	controls                []string
	automationID, className string
	invokable               bool
}

type documentTopology struct {
	tabs   []documentNode
	active string
	editor string
}

const maxDocumentTreeNodes = 1024

func sameDocumentNodes(a, b []documentNode) bool {
	if len(a) != len(b) {
		return false
	}
	byID := map[string]documentNode{}
	for _, node := range a {
		byID[node.key] = node
	}
	for _, node := range b {
		old, ok := byID[node.key]
		if !ok || old.parent != node.parent || old.group != node.group || old.tab != node.tab || old.selected != node.selected || old.selectable != node.selectable || old.text != node.text || old.visible != node.visible || old.protected != node.protected || old.automationID != node.automationID || old.className != node.className || old.invokable != node.invokable || nativeHash(old.controls) != nativeHash(node.controls) {
			return false
		}
	}
	return true
}

func resolveDocumentTopology(nodes []documentNode, complete bool) (documentTopology, error) {
	if !complete || len(nodes) == 0 || len(nodes) > maxDocumentTreeNodes {
		return documentTopology{}, ErrDocumentIncomplete
	}
	byID := make(map[string]documentNode, len(nodes))
	for _, node := range nodes {
		if node.key == "" || len(node.controls) > 16 {
			return documentTopology{}, ErrDocumentIncomplete
		}
		if _, exists := byID[node.key]; exists {
			return documentTopology{}, ErrDocumentChanged
		}
		byID[node.key] = node
	}
	// Every ancestry edge must be from the same bounded tree, and acyclic.
	for _, node := range nodes {
		seen := map[string]bool{node.key: true}
		for parent := node.parent; parent != ""; {
			next, exists := byID[parent]
			if !exists || seen[parent] || len(seen) >= 64 {
				return documentTopology{}, ErrDocumentIncomplete
			}
			seen[parent] = true
			parent = next.parent
		}
	}
	within := func(child, ancestor string) bool {
		for child != "" {
			if child == ancestor {
				return true
			}
			child = byID[child].parent
		}
		return false
	}
	result := documentTopology{}
	group := ""
	for _, node := range nodes {
		if !node.tab {
			continue
		}
		if node.protected || node.group == "" || node.group == node.key {
			return documentTopology{}, ErrDocumentUnavailable
		}
		if _, exists := byID[node.group]; !exists || !within(node.key, node.group) {
			return documentTopology{}, ErrDocumentUnavailable
		}
		if group != "" && group != node.group {
			// Nested settings tabs and document tabs cannot be disambiguated
			// by their labels. A reviewed application profile is required.
			return documentTopology{}, ErrDocumentUnavailable
		}
		group = node.group
		result.tabs = append(result.tabs, node)
		if node.selected {
			if result.active != "" {
				return documentTopology{}, ErrDocumentChanged
			}
			result.active = node.key
		}
	}
	if len(result.tabs) == 0 || len(result.tabs) > MaxDocumentInventory || result.active == "" {
		return documentTopology{}, ErrDocumentUnavailable
	}
	active := byID[result.active]
	panes := append([]string{active.key}, active.controls...)
	for _, pane := range active.controls {
		if _, exists := byID[pane]; !exists || pane == group || within(group, pane) {
			return documentTopology{}, ErrDocumentUnavailable
		}
	}
	var candidates []string
	for _, node := range nodes {
		if !node.text || !node.visible || node.protected {
			continue
		}
		for _, pane := range panes {
			if within(node.key, pane) {
				candidates = append(candidates, node.key)
				break
			}
		}
	}
	// An outer TextPattern can cover inner text controls. Only its complete
	// document is considered; disjoint editors are ambiguous, not concatenated.
	for _, candidate := range candidates {
		nested := false
		for _, other := range candidates {
			if candidate != other && within(candidate, other) {
				nested = true
			}
		}
		if !nested {
			if result.editor != "" {
				return documentTopology{}, ErrDocumentUnavailable
			}
			result.editor = candidate
		}
	}
	if result.editor == "" {
		return documentTopology{}, ErrDocumentUnavailable
	}
	for _, node := range nodes {
		// An outer TextPattern must not bypass a protected descendant just
		// because that child was excluded from the list of editable candidates.
		if node.protected && within(node.key, result.editor) {
			return documentTopology{}, ErrDocumentUnavailable
		}
	}
	// Another tab must not claim the same editor/pane. Shared editor controls
	// require an explicit provider relationship that changes with selection.
	for _, tab := range result.tabs {
		if tab.selected {
			continue
		}
		for _, pane := range append([]string{tab.key}, tab.controls...) {
			if _, exists := byID[pane]; exists && within(result.editor, pane) {
				return documentTopology{}, ErrDocumentUnavailable
			}
		}
	}
	return result, nil
}
