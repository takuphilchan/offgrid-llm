//go:build windows && amd64

package computer

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

// Contract availability is independent of per-surface support and qualification.
var _ NativeDocumentProvider = (*NativeWindows)(nil)

func (*NativeWindows) DocumentContractVersion() int { return NativeDocumentsVersion }

type uiaDocumentState struct {
	target TargetIdentity
	docs   map[string]uiaDocumentIdentity
}

type uiaDocumentIdentity struct {
	id, generation, editor, revision, textHash string
}

type uiaDocumentTree struct {
	nodes    []documentNode
	elements map[string]*ole.IUnknown
	commands []DocumentCreationCommand
	profile  bool
}

func (t *uiaDocumentTree) close() {
	for _, element := range t.elements {
		element.Release()
	}
	t.elements = nil
}

// ControllerFor is the UIA-defined link from a TabItem to a non-child pane.
// https://learn.microsoft.com/windows/win32/winauto/uiauto-supporttabitemcontroltype
func uiaControlledPanes(element *ole.IUnknown) ([]string, error) {
	var value ole.VARIANT
	if err := comCall(element, 10, 30104, uintptr(unsafe.Pointer(&value))); err != nil {
		return nil, err
	}
	defer value.Clear()
	if value.VT == ole.VT_EMPTY {
		return nil, nil
	}
	unknown := value.ToIUnknown()
	if unknown == nil {
		return nil, ErrDocumentUnavailable
	}
	// Unsupported properties may contain UIA's reserved IUnknown sentinel.
	// Query the array interface before using its vtable; never cast the sentinel.
	var array *ole.IUnknown
	if unknown.PutQueryInterface(ole.NewGUID("{14314595-B4BC-4055-95F2-58F2E42C9855}"), &array) != nil || array == nil {
		return nil, nil // A child-pane relationship can still establish identity.
	}
	defer array.Release()
	count, err := comInt(array, 3)
	if err != nil || count < 0 || count > 16 {
		return nil, ErrDocumentIncomplete
	}
	result := make([]string, 0, count)
	for i := int32(0); i < count; i++ {
		var pane *ole.IUnknown
		if err := comCall(array, 4, uintptr(i), uintptr(unsafe.Pointer(&pane))); err != nil || pane == nil {
			return nil, ErrDocumentUnavailable
		}
		id, err := runtimeIdentity(pane)
		pane.Release()
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func uiaTabSelection(element *ole.IUnknown) (bool, string, error) {
	p := pattern(element, 10010) // SelectionItemPattern, not text/keyboard Tab.
	if p == nil {
		return false, "", ErrDocumentUnavailable
	}
	defer p.Release()
	selected, err := comInt(p, 6)
	if err != nil || selected < 0 || selected > 1 {
		return false, "", ErrDocumentUnavailable
	}
	var group *ole.IUnknown
	if comCall(p, 7, uintptr(unsafe.Pointer(&group))) != nil || group == nil {
		return false, "", ErrDocumentUnavailable
	}
	defer group.Release()
	selection := pattern(group, 10001) // SelectionPattern must report a single selection.
	if selection == nil {
		return false, "", ErrDocumentUnavailable
	}
	defer selection.Release()
	multiple, err := comInt(selection, 4)
	if err != nil || multiple != 0 {
		return false, "", ErrDocumentUnavailable
	}
	if selected == 1 {
		var items *ole.IUnknown
		if comCall(selection, 3, uintptr(unsafe.Pointer(&items))) != nil || items == nil {
			return false, "", ErrDocumentUnavailable
		}
		defer items.Release()
		count, err := comInt(items, 3)
		if err != nil || count != 1 {
			return false, "", ErrDocumentChanged
		}
		var item *ole.IUnknown
		if comCall(items, 4, 0, uintptr(unsafe.Pointer(&item))) != nil || item == nil {
			return false, "", ErrDocumentUnavailable
		}
		defer item.Release()
		actual, e1 := runtimeIdentity(item)
		expected, e2 := runtimeIdentity(element)
		if e1 != nil || e2 != nil || actual != expected {
			return false, "", ErrDocumentChanged
		}
	}
	id, err := runtimeIdentity(group)
	return selected == 1, id, err
}

// Walk a bounded control tree. FindAll on an entire application could allocate
// an unbounded result before OffGrid applies its cap. This also records actual
// parent relationships, rather than guessing from a flat list's order.
func (driver *NativeWindows) documentTree(ctx context.Context, binding ControlBinding) (*uiaDocumentTree, error) {
	if binding.Validate() != nil || driver.selected == nil || binding.Target != driver.selected.target.Identity {
		return nil, ErrControlScope
	}
	if err := driver.checkTarget(false); err != nil {
		return nil, err
	}
	tree := &uiaDocumentTree{elements: map[string]*ole.IUnknown{}}
	var root, walker *ole.IUnknown
	if err := comCall(driver.automation, 6, driver.selected.hwnd, uintptr(unsafe.Pointer(&root))); err != nil || root == nil {
		return nil, ErrDocumentUnavailable
	}
	defer root.Release()
	if err := comCall(driver.automation, 16, uintptr(unsafe.Pointer(&walker))); err != nil || walker == nil {
		return nil, ErrDocumentUnavailable
	}
	defer walker.Release()
	var walk func(*ole.IUnknown, string, int, bool) error
	walk = func(element *ole.IUnknown, parent string, depth int, protected bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth >= 64 || len(tree.nodes) >= maxDocumentTreeNodes {
			return ErrDocumentIncomplete
		}
		key, err := runtimeIdentity(element)
		if err != nil || tree.elements[key] != nil {
			return ErrDocumentChanged
		}
		pid, err := comInt(element, 20)
		if err != nil || uint32(pid) != driver.selected.pid {
			return ErrControlScope
		}
		kind, err := comInt(element, 21)
		if err != nil {
			return err
		}
		password, err := comInt(element, 35)
		if err != nil {
			return err
		}
		enabled, err := comInt(element, 28)
		if err != nil {
			return err
		}
		offscreen, err := comInt(element, 38)
		if err != nil {
			return err
		}
		node := documentNode{key: key, parent: parent, protected: protected || password != 0, visible: offscreen == 0}
		if !node.protected {
			node.automationID, _ = comString(element, 29)
			node.className, _ = comString(element, 30)
			if enabled != 0 && node.visible && kind == 50000 {
				if p := pattern(element, 10000); p != nil {
					node.invokable = true
					p.Release()
				}
			}
		}
		if kind == 50019 { // Numeric TabItem role, independent of locale/name.
			node.tab, node.selectable = true, enabled != 0 && offscreen == 0 && !node.protected
			node.title, err = comString(element, 23)
			if err != nil || !utf8.ValidString(node.title) || len(node.title) > 1024 || strings.ContainsRune(node.title, 0) {
				return ErrDocumentIncomplete
			}
			node.selected, node.group, err = uiaTabSelection(element)
			if err != nil {
				return err
			}
			node.controls, err = uiaControlledPanes(element)
			if err != nil {
				return err
			}
		}
		if !node.protected && enabled != 0 && node.visible && (kind == 50030 || kind == 50004) {
			if p := pattern(element, 10014); p != nil {
				node.text = true
				p.Release()
			}
		}
		element.AddRef()
		tree.elements[key] = element
		tree.nodes = append(tree.nodes, node)
		// Never walk into a protected subtree, even to obtain its labels.
		if node.protected {
			return nil
		}
		var child *ole.IUnknown
		if err := comCall(walker, 4, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&child))); err != nil {
			if child != nil {
				child.Release()
			}
			return err
		}
		for child != nil {
			if err := walk(child, key, depth+1, node.protected); err != nil {
				child.Release()
				return err
			}
			var next *ole.IUnknown
			err := comCall(walker, 6, uintptr(unsafe.Pointer(child)), uintptr(unsafe.Pointer(&next)))
			child.Release()
			if err != nil {
				if next != nil {
					next.Release()
				}
				return err
			}
			child = next
		}
		return nil
	}
	if err := walk(root, "", 0, false); err != nil {
		tree.close()
		return nil, err
	}
	// TabView exposes selected content in a sibling ContentPresenter. Never
	// synthesize that relationship for an unknown application or tab control.
	if identity, e := driver.TargetApplication(driver.selected.target); e == nil && notepadDocumentProfile(identity.AUMID, identity.FullName) {
		if nodes, commands, e := resolveWinUIDocumentProfile(tree.nodes, nativeHash(identity)); e == nil {
			tree.nodes, tree.commands = nodes, commands
			tree.profile = true
		}
	}
	return tree, nil
}

func uiaDocumentWritable(editor *ole.IUnknown) bool {
	p, r, err := uiaDocumentRange(editor)
	if err != nil {
		return false
	}
	defer p.Release()
	defer r.Release()
	var value ole.VARIANT
	defer value.Clear()
	if comCall(r, 8, 40015, uintptr(unsafe.Pointer(&value))) != nil { // IsReadOnly text attribute
		return false
	}
	return value.VT == ole.VT_BOOL && value.Val == 0
}

func (driver *NativeWindows) ReadDocument(ctx context.Context, binding ControlBinding) (inventory DocumentInventory, snapshot DocumentSnapshot, err error) {
	// Unknown continuity invalidates all cached identities, not just the active
	// editor. The next successful inventory cannot revive older preparations.
	defer func() {
		if err != nil {
			driver.documentState = nil
		}
	}()
	tree, err := driver.documentTree(ctx, binding)
	if err != nil {
		return inventory, snapshot, err
	}
	defer tree.close()
	topology, err := resolveDocumentTopology(tree.nodes, true)
	if err != nil {
		return inventory, snapshot, err
	}
	editor := tree.elements[topology.editor]
	text, selection, composition, err := readUIADocument(editor)
	if err != nil {
		return inventory, snapshot, err
	}
	// Recheck relationships after collecting text. A user tab switch during
	// the read must not pair one tab's identity with another tab's text.
	current, err := driver.documentTree(ctx, binding)
	if err != nil {
		return inventory, snapshot, err
	}
	defer current.close()
	if !sameDocumentTree(tree.nodes, current.nodes) {
		return inventory, snapshot, ErrDocumentChanged
	}
	if err := driver.checkTarget(false); err != nil {
		return inventory, snapshot, err
	}
	state := driver.documentState
	if state == nil || state.target != binding.Target {
		state = &uiaDocumentState{target: binding.Target, docs: map[string]uiaDocumentIdentity{}}
	}
	next := &uiaDocumentState{target: state.target, docs: map[string]uiaDocumentIdentity{}}
	driver.sequence++
	observation := ControlObservation{ID: nativeID(), Target: binding.Target, Sequence: driver.sequence, CapturedAt: time.Now()}
	inventory = DocumentInventory{Version: NativeDocumentsVersion, Observation: observation, Complete: true, Documents: []NativeDocument{}, Commands: append([]DocumentCreationCommand{}, tree.commands...)}
	for _, tab := range topology.tabs {
		identity, exists := state.docs[tab.key]
		if !exists {
			identity = uiaDocumentIdentity{id: nativeID(), generation: nativeID()}
		}
		caps := DocumentCapabilities{Select: tab.selectable}
		if tab.key == topology.active {
			if identity.editor != "" && identity.editor != topology.editor {
				identity.generation = nativeID()
			}
			identity.editor = topology.editor
			digest := DocumentTextDigest(text)
			if identity.revision == "" || identity.textHash != digest {
				identity.revision, identity.textHash = nativeID(), digest
			}
			bound := DocumentBinding{Target: binding.Target, Document: identity.id, Generation: identity.generation, Container: tab.key, Editor: topology.editor, Observation: observation.ID, Revision: identity.revision}
			inventory.Active = &bound
			snapshot = DocumentSnapshot{ID: nativeID(), Binding: bound, Text: text, Complete: true, Selection: selection, Composition: composition}
			caps.Read = true
			focused, focusErr := comInt(editor, 26)
			stable := composition == "stable" && selection != nil && focusErr == nil && focused == 1
			caps.SelectText = stable
			writable := stable && uiaDocumentWritable(editor)
			caps.InsertText, caps.ReplaceSelection = writable, writable
			if p := pattern(editor, 10002); p != nil {
				readOnly, e := comInt(p, 5)
				caps.ReplaceValue = stable && e == nil && readOnly == 0 && caps.InsertText
				p.Release()
			}
		}
		next.docs[tab.key] = identity
		inventory.Documents = append(inventory.Documents, NativeDocument{ID: identity.id, Generation: identity.generation, Container: tab.key, Title: tab.title, Selected: tab.selected, Capabilities: caps})
	}
	inventory.Observation.StateDigest = nativeHash([]any{inventory.Documents, inventory.Commands, inventory.Active, DocumentTextDigest(text), selection, composition})
	if inventory.Validate(binding.Target, time.Now()) != nil || snapshot.Validate() != nil {
		return DocumentInventory{}, DocumentSnapshot{}, ErrDocumentIncomplete
	}
	driver.documentState = next
	return inventory, snapshot, nil
}

func sameDocumentTree(a, b []documentNode) bool {
	if len(a) != len(b) {
		return false
	}
	byID := make(map[string]documentNode, len(a))
	for _, node := range a {
		byID[node.key] = node
	}
	for _, node := range b {
		old, exists := byID[node.key]
		if !exists || old.parent != node.parent || old.group != node.group || old.tab != node.tab || old.selected != node.selected || old.selectable != node.selectable || old.text != node.text || old.visible != node.visible || old.protected != node.protected || old.automationID != node.automationID || old.className != node.className || old.invokable != node.invokable || nativeHash(old.controls) != nativeHash(node.controls) {
			return false
		}
	}
	return true
}

func (driver *NativeWindows) ApplyDocumentText(ctx context.Context, binding ControlBinding, plan PreparedDocumentText) error {
	inventory, snapshot, err := driver.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	before := plan.Preimage()
	if inventory.Active == nil || ValidateDocumentPrecondition(before, snapshot) != nil {
		return ErrDocumentChanged
	}
	tree, err := driver.documentTree(ctx, binding)
	if err != nil {
		return err
	}
	defer tree.close()
	editor := tree.elements[before.Binding.Editor]
	var activeTab documentNode
	for _, node := range tree.nodes {
		if node.key == before.Binding.Container {
			activeTab = node
		}
	}
	return applyUIADocumentText(ctx, editor, plan, func() error {
		if err := driver.checkTarget(true); err != nil {
			return err
		}
		// Revalidate live selection and ancestry, not a full application tree
		// for every 64-unit batch. This does not refresh the prepared identity.
		if tree.profile {
			current, e := driver.documentTree(ctx, binding)
			if e != nil {
				return e
			}
			defer current.close()
			if !sameDocumentTree(tree.nodes, current.nodes) || nativeHash(tree.commands) != nativeHash(current.commands) {
				return ErrDocumentChanged
			}
			return nil
		}
		return driver.checkDocumentRelation(editor, tree.elements[activeTab.key], activeTab, before.Binding.Editor)
	})
}

func (driver *NativeWindows) checkDocumentRelation(editor, tab *ole.IUnknown, expected documentNode, editorKey string) error {
	if editor == nil || tab == nil || !expected.tab || !expected.selected {
		return ErrDocumentChanged
	}
	id, err := runtimeIdentity(editor)
	if err != nil || id != editorKey {
		return ErrDocumentChanged
	}
	selected, group, err := uiaTabSelection(tab)
	if err != nil || !selected || group != expected.group {
		return ErrDocumentChanged
	}
	panes, err := uiaControlledPanes(tab)
	if err != nil || nativeHash(panes) != nativeHash(expected.controls) {
		return ErrDocumentChanged
	}
	allowed := map[string]bool{expected.key: true}
	for _, pane := range panes {
		allowed[pane] = true
	}
	var walker *ole.IUnknown
	if comCall(driver.automation, 14, uintptr(unsafe.Pointer(&walker))) != nil || walker == nil {
		return ErrDocumentUnavailable
	}
	defer walker.Release()
	editor.AddRef()
	current := editor
	matched := false
	for depth := 0; current != nil && depth < 64; depth++ {
		key, err := runtimeIdentity(current)
		if err != nil {
			current.Release()
			return err
		}
		if allowed[key] {
			matched = true
		}
		var parent *ole.IUnknown
		err = comCall(walker, 3, uintptr(unsafe.Pointer(current)), uintptr(unsafe.Pointer(&parent)))
		current.Release()
		if err != nil {
			if parent != nil {
				parent.Release()
			}
			return err
		}
		current = parent
	}
	if current != nil {
		current.Release()
		return ErrDocumentIncomplete
	}
	if !matched || !driver.withinTarget(editor) || !driver.withinTarget(tab) {
		return ErrDocumentChanged
	}
	return nil
}

func (driver *NativeWindows) ApplyDocumentLifecycle(ctx context.Context, binding ControlBinding, before DocumentInventory, op DocumentOperation) error {
	if op.Validate() != nil || (op.Kind != "select_document" && op.Kind != "create_document") {
		return ErrDocumentUnavailable
	}
	current, _, err := driver.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	if !sameDocumentInventory(before, current) || driver.checkTarget(true) != nil {
		return ErrDocumentChanged
	}
	key := ""
	for _, doc := range current.Documents {
		if doc.ID == op.Document && doc.Capabilities.Select {
			key = doc.Container
		}
	}
	tree, err := driver.documentTree(ctx, binding)
	if err != nil {
		return err
	}
	defer tree.close()
	if op.Kind == "create_document" {
		for _, command := range before.Commands {
			if command.ID != op.Command {
				continue
			}
			for _, live := range tree.commands {
				if live != command {
					continue
				}
				element := tree.elements[live.Element]
				if element == nil || !driver.withinTarget(element) {
					return ErrDocumentChanged
				}
				p := pattern(element, 10000)
				if p == nil {
					return ErrDocumentUnavailable
				}
				defer p.Release()
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := driver.checkTarget(true); err != nil {
					return err
				}
				return comCall(p, 3) // Invoke once; executor waits read-only for a distinct blank document.
			}
		}
		return ErrDocumentChanged
	}
	topology, err := resolveDocumentTopology(tree.nodes, true)
	if err != nil || current.Active == nil || topology.active != current.Active.Container || topology.editor != current.Active.Editor || len(topology.tabs) != len(current.Documents) {
		return ErrDocumentChanged
	}
	for _, tab := range topology.tabs {
		found := false
		for _, doc := range current.Documents {
			if doc.Container == tab.key && doc.Selected == tab.selected && doc.Capabilities.Select == tab.selectable {
				found = true
			}
		}
		if !found {
			return ErrDocumentChanged
		}
	}
	if key == "" || tree.elements[key] == nil {
		return ErrDocumentChanged
	}
	p := pattern(tree.elements[key], 10010)
	if p == nil {
		return ErrDocumentUnavailable
	}
	defer p.Release()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := driver.checkTarget(true); err != nil {
		return err
	}
	return comCall(p, 3) // SelectionItem.Select once; executor verifies actual selection.
}
