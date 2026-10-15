package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// One document workspace per owned worker, not a second grant or dispatcher.
// The caller must check its existing supervisor/desktop authority before reads;
// input is called exclusively from that supervisor's durable dispatch callback.
type documentWorker struct {
	executor  *computer.NativeDocumentExecutor
	binding   computer.ControlBinding
	inventory computer.DocumentInventory
	snapshot  computer.DocumentSnapshot
}

func newDocumentWorker(driver computer.NativeAdapter) *documentWorker {
	backend, ok := driver.(computer.NativeDocumentBackend)
	if !ok || computer.ProviderNativeFeatures(driver).Documents != computer.NativeDocumentsVersion {
		return nil
	}
	return &documentWorker{executor: computer.NewNativeDocumentExecutor(backend)}
}

func (d *documentWorker) clear() {
	if d == nil {
		return
	}
	d.executor.Clear()
	d.binding = computer.ControlBinding{}
	d.inventory = computer.DocumentInventory{}
	d.snapshot = computer.DocumentSnapshot{}
}

func (d *documentWorker) documents(ctx context.Context, binding computer.ControlBinding) (computer.DocumentInventory, error) {
	if d == nil {
		return computer.DocumentInventory{}, computer.ErrDocumentUnavailable
	}
	d.clear()
	inventory, snapshot, err := d.executor.Read(ctx, binding)
	if err != nil {
		return computer.DocumentInventory{}, err
	}
	d.binding, d.inventory, d.snapshot = binding, inventory, snapshot
	return inventory, nil
}

// A fresh provider read must agree with the inventory epoch. A second read is
// not allowed to silently switch the selection/revision and make old IDs usable.
func (d *documentWorker) read(ctx context.Context, binding computer.ControlBinding, observed computer.ControlObservation, requests ...*computer.DocumentPageRequest) (computer.DocumentReadback, error) {
	if d == nil {
		return computer.DocumentReadback{}, computer.ErrDocumentUnavailable
	}
	if binding != d.binding || observed != d.inventory.Observation || observed.CapturedAt.After(time.Now()) || observed.Validate(binding.Target, observed.CapturedAt) != nil || d.inventory.Active == nil {
		return computer.DocumentReadback{}, computer.ErrStaleObservation
	}
	var page *computer.DocumentPageRequest
	if len(requests) > 1 {
		return computer.DocumentReadback{}, computer.ErrInvalidControl
	}
	if len(requests) == 1 {
		page = requests[0]
	}
	if page != nil && (page.Validate() != nil || page.Snapshot != d.snapshot.ID || page.Revision != d.snapshot.Binding.Revision) {
		return computer.DocumentReadback{}, computer.ErrStaleObservation
	}
	inventory, current, err := d.executor.ReadVerification(ctx, binding)
	if err != nil {
		return computer.DocumentReadback{}, err
	}
	if inventory.Active == nil || computer.ValidateDocumentPrecondition(d.snapshot, current) != nil {
		return computer.DocumentReadback{}, computer.ErrDocumentChanged
	}
	// This fresh read substantiates the unchanged, still-current inventory
	// epoch. Keep its reference stable for model arguments; the snapshot is new.
	current.Binding.Observation = observed.ID
	start := 0
	if page == nil {
		d.snapshot = current
	} else {
		start = page.Start
	}
	pager, err := computer.NewDocumentPager(d.snapshot)
	if err != nil {
		return computer.DocumentReadback{}, err
	}
	part, err := pager.Page(d.snapshot.ID, d.snapshot.Binding.Revision, start, current)
	if err != nil {
		return computer.DocumentReadback{}, err
	}
	result, err := computer.NewDocumentReadback(d.snapshot)
	if err != nil {
		return computer.DocumentReadback{}, err
	}
	result.Page = &part
	if d.snapshot.Selection != nil {
		selection := *d.snapshot.Selection
		result.Selection = &selection
	}
	return result, result.Validate()
}

func (d *documentWorker) prepare(ctx context.Context, binding computer.ControlBinding, op computer.Operation, observed computer.ControlObservation) (computer.PreparedControlAction, error) {
	if d == nil {
		return computer.PreparedControlAction{}, computer.ErrDocumentUnavailable
	}
	if binding != d.binding || observed != d.inventory.Observation || observed.CapturedAt.After(time.Now()) || observed.Validate(binding.Target, observed.CapturedAt) != nil {
		return computer.PreparedControlAction{}, computer.ErrStaleObservation
	}
	if op.Kind != "document" || op.Validate() != nil {
		return computer.PreparedControlAction{}, computer.ErrInvalidControl
	}
	return d.executor.PrepareCurrent(ctx, binding, *op.Document, d.inventory, d.snapshot)
}

func (d *documentWorker) dispatch(ctx context.Context, a computer.PreparedControlAction) (computer.ControlResult, error) {
	if d == nil {
		return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, computer.ErrDocumentUnavailable
	}
	defer d.clear()
	return d.executor.Dispatch(ctx, a)
}

func (d *documentWorker) summary(step computer.BoundedStep) (string, error) {
	if d == nil || step.Binding != d.binding || len(step.Actions) != 1 {
		return "", computer.ErrControlScope
	}
	a := step.Actions[0]
	if a.Operation.Kind != "document" || a.Operation.Validate() != nil {
		return "", computer.ErrInvalidControl
	}
	op := a.Operation.Document
	// Called only after matching against the worker's immutable preparation.
	// No title/URL from a model or document is used as permission language.
	message := "Allow this document action in the selected application?\n\n"
	switch op.Kind {
	case "create_document":
		message += "Create one new blank document or tab. Existing documents will not be cleared, saved or closed."
	case "select_document":
		message += "Select the exact observed document."
	case "select_text":
		message += fmt.Sprintf("Select Unicode scalar range %d to %d in the observed document.", op.Range.Start, op.Range.End)
	case "insert_text":
		message += "Insert at the verified caret:\n" + strconv.Quote(*op.Text)
	case "replace_selection":
		message += "Replace only the verified selection:\n" + strconv.Quote(*op.Text)
	case "replace_value":
		message += "Replace the document's whole value:\n" + strconv.Quote(*op.Text)
	default:
		return "", computer.ErrInvalidControl
	}
	return message + "\n\nNo save or other action is included. Stop remains available in OffGrid.", nil
}
