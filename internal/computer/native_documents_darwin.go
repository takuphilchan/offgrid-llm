//go:build darwin && cgo

package computer

import (
	"context"
	"strings"
	"unicode/utf8"
)

type axDocumentTreeNode struct {
	Key        string   `json:"key"`
	Parent     string   `json:"parent"`
	Group      string   `json:"group"`
	Title      string   `json:"title"`
	Tab        bool     `json:"tab"`
	Selected   bool     `json:"selected"`
	Selectable bool     `json:"selectable"`
	Text       bool     `json:"text"`
	Visible    bool     `json:"visible"`
	Protected  bool     `json:"protected"`
	Controls   []string `json:"controls"`
}

func axDocumentTree(ctx context.Context) ([]documentNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var raw []axDocumentTreeNode
	if err := axCall("document_tree", nil, &raw); err != nil {
		return nil, err
	}
	nodes := make([]documentNode, 0, len(raw))
	for _, n := range raw {
		nodes = append(nodes, documentNode{key: n.Key, parent: n.Parent, group: n.Group, title: n.Title, tab: n.Tab, selected: n.Selected, selectable: n.Selectable, text: n.Text, visible: n.Visible, protected: n.Protected, controls: n.Controls})
	}
	return nodes, nil
}

type axDocumentText struct {
	Text             string            `json:"text"`
	Selection        DocumentTextRange `json:"selection"`
	SelectedText     string            `json:"selected_text"`
	Composition      string            `json:"composition"`
	SelectText       bool              `json:"select_text"`
	ReplaceSelection bool              `json:"replace_selection"`
	ReplaceValue     bool              `json:"replace_value"`
	Focused          bool              `json:"focused"`
}

var _ NativeDocumentProvider = (*NativeDarwin)(nil)

func (*NativeDarwin) DocumentContractVersion() int { return NativeDocumentsVersion }

func (d *NativeDarwin) ReadDocument(ctx context.Context, binding ControlBinding) (inv DocumentInventory, snap DocumentSnapshot, err error) {
	defer func() {
		if err != nil {
			d.documents = nil
		}
	}()
	if binding.Validate() != nil || binding.Target != d.selected.Identity {
		return inv, snap, ErrControlScope
	}
	nodes, err := axDocumentTree(ctx)
	if err != nil {
		return inv, snap, err
	}
	topology, err := resolveDocumentTopology(nodes, true)
	if err != nil {
		return inv, snap, err
	}
	var read axDocumentText
	if err = axCall("document_text", map[string]string{"editor": topology.editor}, &read); err != nil {
		return inv, snap, err
	}
	if !utf8.ValidString(read.Text) || strings.ContainsRune(read.Text, 0) || len(read.Text) > MaxDocumentSnapshotBytes {
		return inv, snap, ErrDocumentIncomplete
	}
	r, err := DocumentRangeFromNative(read.Text, read.Selection, TextUnitUTF16)
	if err != nil {
		return inv, snap, err
	}
	bytes, err := DocumentRangeToNative(read.Text, r, TextUnitUTF8)
	if err != nil || read.Text[bytes.Start:bytes.End] != read.SelectedText {
		return inv, snap, ErrDocumentRange
	}
	current, err := axDocumentTree(ctx)
	if err != nil || !sameDocumentNodes(nodes, current) {
		return inv, snap, ErrDocumentChanged
	}
	var second axDocumentText
	if err = axCall("document_text", map[string]string{"editor": topology.editor}, &second); err != nil || second != read {
		return inv, snap, ErrDocumentChanged
	}
	stable := read.Composition == "stable" && read.Focused
	caps := DocumentCapabilities{Read: true, SelectText: stable && read.SelectText, InsertText: stable && read.ReplaceSelection, ReplaceSelection: stable && read.ReplaceSelection, ReplaceValue: stable && read.ReplaceValue}
	d.documentSequence++
	inv, snap, d.documents, err = d.documents.inventory(binding.Target, topology, read.Text, &r, read.Composition, caps, nil, d.documentSequence)
	return
}

func (d *NativeDarwin) ApplyDocumentLifecycle(ctx context.Context, binding ControlBinding, before DocumentInventory, op DocumentOperation) error {
	if op.Validate() != nil || op.Kind != "select_document" {
		return ErrDocumentUnavailable
	}
	current, _, err := d.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	if !sameDocumentInventory(before, current) {
		return ErrDocumentChanged
	}
	for _, doc := range current.Documents {
		if doc.ID == op.Document && doc.Capabilities.Select {
			if err := ctx.Err(); err != nil {
				return err
			}
			return axCall("document_select", map[string]any{"document": doc.Container}, nil)
		}
	}
	return ErrDocumentChanged
}

func (d *NativeDarwin) ApplyDocumentText(ctx context.Context, binding ControlBinding, plan PreparedDocumentText) error {
	_, snap, err := d.ReadDocument(ctx, binding)
	if err != nil {
		return err
	}
	if err := ValidateDocumentPrecondition(plan.Preimage(), snap); err != nil {
		return err
	}
	op := plan.Operation()
	selection, err := DocumentRangeToNative(snap.Text, *snap.Selection, TextUnitUTF16)
	if err != nil {
		return err
	}
	args := map[string]any{"editor": snap.Binding.Editor, "document": snap.Binding.Container, "preimage": snap.Text, "selection": selection, "kind": op.Kind}
	if op.Kind == "select_text" {
		r, e := DocumentRangeToNative(snap.Text, *op.Range, TextUnitUTF16)
		if e != nil {
			return e
		}
		args["range"] = r
	} else {
		args["text"] = *op.Text
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return axCall("document_edit", args, nil)
}
