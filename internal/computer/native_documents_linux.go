//go:build linux && cgo && offgrid_native

package computer

/*
#cgo pkg-config: atspi-2 gobject-2.0
#include <atspi/atspi.h>
#include <stdlib.h>
static void doc_unref(void *p){if(p)g_object_unref(p);}
static void doc_ref(void *p){if(p)g_object_ref(p);}
static int doc_state(AtspiAccessible *p,AtspiStateType state){
 AtspiStateSet *s=atspi_accessible_get_state_set(p);if(!s)return 0;
 int answer=atspi_state_set_contains(s,state);g_object_unref(s);return answer;
}
*/
import "C"

import (
	"context"
	"strings"
	"unicode/utf8"
	"unsafe"
)

type linuxDocumentReferences struct {
	objects map[string]*C.AtspiAccessible
	state   *nativeDocumentState
}

var _ NativeDocumentProvider = (*NativeLinux)(nil)

func (*NativeLinux) DocumentContractVersion() int { return NativeDocumentsVersion }

func (d *NativeLinux) clearDocumentReferences() {
	if d.documents != nil {
		for _, object := range d.documents.objects {
			C.doc_unref(unsafe.Pointer(object))
		}
	}
	d.documents = nil
}

func (d *NativeLinux) documentTree(ctx context.Context) ([]documentNode, map[string]*C.AtspiAccessible, error) {
	if err := d.check(false); err != nil {
		return nil, nil, err
	}
	objects := map[string]*C.AtspiAccessible{}
	ids := map[*C.AtspiAccessible]string{}
	if d.documents != nil {
		for key, object := range d.documents.objects {
			ids[object] = key
		}
	}
	keyFor := func(object *C.AtspiAccessible) string {
		if key := ids[object]; key != "" {
			return key
		}
		key := nativeID()
		ids[object] = key
		return key
	}
	nodes := []documentNode{}
	var walk func(*C.AtspiAccessible, string, int) error
	walk = func(object *C.AtspiAccessible, parent string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(nodes) >= maxDocumentTreeNodes || depth >= 64 {
			return ErrDocumentIncomplete
		}
		C.atspi_accessible_clear_cache(object)
		var e *C.GError
		pid := C.atspi_accessible_get_process_id(object, &e)
		if atspiError(e) || int(pid) != d.selected.pid {
			return ErrControlScope
		}
		key := keyFor(object)
		if objects[key] != nil {
			return ErrDocumentChanged
		}
		C.doc_ref(unsafe.Pointer(object))
		objects[key] = object
		e = nil
		role := C.atspi_accessible_get_role(object, &e)
		if atspiError(e) {
			return ErrDocumentUnavailable
		}
		n := documentNode{key: key, parent: parent, visible: C.doc_state(object, C.ATSPI_STATE_SHOWING) != 0, protected: role == C.ATSPI_ROLE_PASSWORD_TEXT}
		if C.doc_state(object, C.ATSPI_STATE_DEFUNCT) != 0 {
			return ErrDocumentChanged
		}
		if n.protected {
			nodes = append(nodes, n)
			return nil
		}
		if role == C.ATSPI_ROLE_PAGE_TAB {
			n.tab = true
			n.selected = C.doc_state(object, C.ATSPI_STATE_SELECTED) != 0
			n.selectable = C.doc_state(object, C.ATSPI_STATE_SELECTABLE) != 0 && C.doc_state(object, C.ATSPI_STATE_ENABLED) != 0
			n.group = parent
			e = nil
			n.title = atspiString(C.atspi_accessible_get_name(object, &e))
			if atspiError(e) || len(n.title) > 1024 || !utf8.ValidString(n.title) || strings.ContainsRune(n.title, 0) {
				return ErrDocumentIncomplete
			}
		}
		if n.visible && (role == C.ATSPI_ROLE_TEXT || role == C.ATSPI_ROLE_DOCUMENT_TEXT || role == C.ATSPI_ROLE_ENTRY) {
			p := C.atspi_accessible_get_text_iface(object)
			if p != nil {
				n.text = true
				C.doc_unref(unsafe.Pointer(p))
			}
		}
		nodes = append(nodes, n)
		e = nil
		count := C.atspi_accessible_get_child_count(object, &e)
		if atspiError(e) || count < 0 || int(count) > maxDocumentTreeNodes-len(nodes) {
			return ErrDocumentIncomplete
		}
		for i := C.int(0); i < count; i++ {
			e = nil
			child := C.atspi_accessible_get_child_at_index(object, i, &e)
			if atspiError(e) || child == nil {
				return ErrDocumentIncomplete
			}
			err := walk(child, key, depth+1)
			C.doc_unref(unsafe.Pointer(child))
			if err != nil {
				return err
			}
		}
		return nil
	}
	err := walk(d.selected.object, "", 0)
	if err != nil {
		for _, object := range objects {
			C.doc_unref(unsafe.Pointer(object))
		}
		return nil, nil, err
	}
	return nodes, objects, nil
}

func linuxDocumentText(object *C.AtspiAccessible) (string, *DocumentTextRange, error) {
	p := C.atspi_accessible_get_text_iface(object)
	if p == nil {
		return "", nil, ErrDocumentUnavailable
	}
	defer C.doc_unref(unsafe.Pointer(p))
	var e *C.GError
	count := C.atspi_text_get_character_count(p, &e)
	if atspiError(e) || count < 0 || count > MaxDocumentSnapshotBytes {
		return "", nil, ErrDocumentIncomplete
	}
	e = nil
	text := atspiString(C.atspi_text_get_text(p, 0, count, &e))
	if atspiError(e) || !utf8.ValidString(text) || strings.ContainsRune(text, 0) || len(text) > MaxDocumentSnapshotBytes || utf8.RuneCountInString(text) != int(count) {
		return "", nil, ErrDocumentIncomplete
	}
	e = nil
	selections := C.atspi_text_get_n_selections(p, &e)
	if atspiError(e) || selections < 0 || selections > 1 {
		return "", nil, ErrDocumentRange
	}
	var selection DocumentTextRange
	if selections == 1 {
		e = nil
		r := C.atspi_text_get_selection(p, 0, &e)
		if atspiError(e) || r == nil {
			return "", nil, ErrDocumentRange
		}
		selection = DocumentTextRange{Start: int(r.start_offset), End: int(r.end_offset)}
		C.g_free(C.gpointer(unsafe.Pointer(r)))
	} else {
		e = nil
		caret := C.atspi_text_get_caret_offset(p, &e)
		if atspiError(e) {
			return "", nil, ErrDocumentRange
		}
		selection = DocumentTextRange{Start: int(caret), End: int(caret)}
	}
	if _, err := DocumentRangeFromNative(text, selection, TextUnitScalar); err != nil {
		return "", nil, err
	}
	return text, &selection, nil
}

// AT-SPI has no standard IME-composition state. Absence of such a property is
// not proof that a user's composition is finished. A reviewed toolkit profile
// must supply that evidence before any edit is advertised or dispatched.
func linuxDocumentComposition(*C.AtspiAccessible) string { return "unknown" }

func (d *NativeLinux) ReadDocument(ctx context.Context, binding ControlBinding) (inv DocumentInventory, snap DocumentSnapshot, err error) {
	defer func() {
		if err != nil {
			d.clearDocumentReferences()
		}
	}()
	if binding.Validate() != nil || d.selected == nil || binding.Target != d.selected.target.Identity {
		return inv, snap, ErrControlScope
	}
	nodes, objects, err := d.documentTree(ctx)
	if err != nil {
		return inv, snap, err
	}
	defer func() {
		if objects != nil {
			for _, object := range objects {
				C.doc_unref(unsafe.Pointer(object))
			}
		}
	}()
	topology, err := resolveDocumentTopology(nodes, true)
	if err != nil {
		return inv, snap, err
	}
	editor := objects[topology.editor]
	text, selection, err := linuxDocumentText(editor)
	if err != nil {
		return inv, snap, err
	}
	// Retain the first capture during the second traversal so equal native
	// objects keep their references even on the very first read.
	old := d.documents
	d.documents = &linuxDocumentReferences{objects: objects}
	current, latest, err := d.documentTree(ctx)
	d.documents = old
	for _, object := range latest {
		C.doc_unref(unsafe.Pointer(object))
	}
	if err != nil || !sameDocumentNodes(nodes, current) {
		return inv, snap, ErrDocumentChanged
	}
	second, r, err := linuxDocumentText(editor)
	if err != nil || second != text || *r != *selection || d.check(false) != nil {
		return inv, snap, ErrDocumentChanged
	}
	composition := linuxDocumentComposition(editor)
	stable := composition == "stable" && C.doc_state(editor, C.ATSPI_STATE_FOCUSED) != 0
	writable := C.doc_state(editor, C.ATSPI_STATE_EDITABLE) != 0
	edit := C.atspi_accessible_get_editable_text_iface(editor)
	writable = writable && edit != nil
	if edit != nil {
		C.doc_unref(unsafe.Pointer(edit))
	}
	caps := DocumentCapabilities{Read: true, SelectText: stable, InsertText: stable && writable, ReplaceSelection: stable && writable, ReplaceValue: stable && writable}
	var state *nativeDocumentState
	if old != nil {
		state = old.state
	}
	d.sequence++
	inv, snap, state, err = state.inventory(binding.Target, topology, text, selection, composition, caps, nil, d.sequence)
	if err != nil {
		return inv, snap, err
	}
	d.clearDocumentReferences()
	d.documents = &linuxDocumentReferences{objects: objects, state: state}
	objects = nil
	return inv, snap, nil
}

func (d *NativeLinux) ApplyDocumentLifecycle(ctx context.Context, binding ControlBinding, before DocumentInventory, op DocumentOperation) error {
	if op.Validate() != nil || op.Kind != "select_document" {
		return ErrDocumentUnavailable
	}
	current, _, err := d.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	if !sameDocumentInventory(before, current) || d.check(true) != nil {
		return ErrDocumentChanged
	}
	for _, doc := range current.Documents {
		if doc.ID != op.Document || !doc.Capabilities.Select {
			continue
		}
		object := d.documents.objects[doc.Container]
		if object == nil || !d.within(object) {
			return ErrDocumentChanged
		}
		var e *C.GError
		parent := C.atspi_accessible_get_parent(object, &e)
		if atspiError(e) || parent == nil {
			return ErrDocumentUnavailable
		}
		defer C.doc_unref(unsafe.Pointer(parent))
		selection := C.atspi_accessible_get_selection_iface(parent)
		if selection == nil {
			return ErrDocumentUnavailable
		}
		defer C.doc_unref(unsafe.Pointer(selection))
		e = nil
		index := C.atspi_accessible_get_index_in_parent(object, &e)
		if atspiError(e) || index < 0 {
			return ErrDocumentChanged
		}
		e = nil
		resolved := C.atspi_accessible_get_child_at_index(parent, index, &e)
		if atspiError(e) || resolved == nil {
			return ErrDocumentChanged
		}
		same := resolved == object
		C.doc_unref(unsafe.Pointer(resolved))
		if !same {
			return ErrDocumentChanged
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.check(true) != nil {
			return ErrControlScope
		}
		e = nil
		if C.atspi_selection_select_child(selection, index, &e) == 0 || atspiError(e) {
			return ErrUncertain
		}
		return nil
	}
	return ErrDocumentChanged
}

func (d *NativeLinux) ApplyDocumentText(ctx context.Context, binding ControlBinding, plan PreparedDocumentText) error {
	_, snap, err := d.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	if err := ValidateDocumentPrecondition(plan.Preimage(), snap); err != nil {
		return err
	}
	editor := d.documents.objects[snap.Binding.Editor]
	if editor == nil || !d.within(editor) || d.check(true) != nil || C.doc_state(editor, C.ATSPI_STATE_FOCUSED) == 0 {
		return ErrControlScope
	}
	if linuxDocumentComposition(editor) != "stable" {
		return ErrDocumentComposition
	}
	C.doc_ref(unsafe.Pointer(editor))
	defer C.doc_unref(unsafe.Pointer(editor))
	p := C.atspi_accessible_get_text_iface(editor)
	if p == nil {
		return ErrDocumentUnavailable
	}
	defer C.doc_unref(unsafe.Pointer(p))
	op := plan.Operation()
	var e *C.GError
	if err := ctx.Err(); err != nil {
		return err
	}
	if op.Kind == "select_text" {
		r, err := DocumentRangeToNative(snap.Text, *op.Range, TextUnitScalar)
		if err != nil {
			return err
		}
		var ok C.gboolean
		if r.Start == r.End {
			ok = C.atspi_text_set_caret_offset(p, C.int(r.Start), &e)
		} else {
			n := C.atspi_text_get_n_selections(p, &e)
			if atspiError(e) {
				return ErrDocumentChanged
			}
			e = nil
			if n == 0 {
				ok = C.atspi_text_add_selection(p, C.int(r.Start), C.int(r.End), &e)
			} else {
				ok = C.atspi_text_set_selection(p, 0, C.int(r.Start), C.int(r.End), &e)
			}
		}
		if ok == 0 || atspiError(e) {
			return ErrUncertain
		}
		return nil
	}
	edit := C.atspi_accessible_get_editable_text_iface(editor)
	if edit == nil {
		return ErrDocumentUnavailable
	}
	defer C.doc_unref(unsafe.Pointer(edit))
	text := C.CString(*op.Text)
	defer C.free(unsafe.Pointer(text))
	if op.Kind == "replace_value" {
		if C.atspi_editable_text_set_text_contents(edit, text, &e) == 0 || atspiError(e) {
			return ErrUncertain
		}
		return nil
	}
	if op.Kind != "insert_text" && op.Kind != "replace_selection" {
		return ErrInvalidControl
	}
	r := *snap.Selection
	if op.Kind == "replace_selection" {
		// The supervisor already committed the one mutation intent. Once this
		// first call starts, EVERY later failure is uncertain; never restore,
		// retry deletion or switch to SetTextContents.
		if C.atspi_editable_text_delete_text(edit, C.int(r.Start), C.int(r.End), &e) == 0 || atspiError(e) {
			return ErrUncertain
		}
		if ctx.Err() != nil || d.check(true) != nil || !d.within(editor) || C.doc_state(editor, C.ATSPI_STATE_FOCUSED) == 0 {
			return ErrUncertain
		}
		_, afterDelete, readErr := d.ReadDocument(ctx, binding)
		if readErr != nil || !SameDocument(snap.Binding, afterDelete.Binding) || afterDelete.Composition != "stable" {
			return ErrUncertain
		}
		pre, sel, err := linuxDocumentText(editor)
		bytes, _ := DocumentRangeToNative(snap.Text, r, TextUnitUTF8)
		if err != nil || pre != snap.Text[:bytes.Start]+snap.Text[bytes.End:] || sel.Start != r.Start || sel.End != r.Start {
			return ErrUncertain
		}
	}
	e = nil
	if C.atspi_editable_text_insert_text(edit, C.int(r.Start), text, C.int(len(*op.Text)), &e) == 0 || atspiError(e) {
		return ErrUncertain
	}
	return nil
}
