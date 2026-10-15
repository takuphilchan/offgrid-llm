package computer

import (
	"context"
	"time"
)

// DocumentOperation is the negotiated host operation union. References are
// resolved from the host's last observation, never from caller-supplied bindings,
// expected digests, native ranges, executable names or document titles.
type DocumentOperation struct {
	Kind     string             `json:"kind"`
	Document string             `json:"document,omitempty"`
	Command  string             `json:"command,omitempty"`
	Snapshot string             `json:"snapshot,omitempty"`
	Range    *DocumentTextRange `json:"range,omitempty"`
	Text     *string            `json:"text,omitempty"`
}

func (op DocumentOperation) Validate() error {
	want := DocumentOperation{Kind: op.Kind}
	switch op.Kind {
	case "create_document":
		if !opaqueID(op.Command) {
			return ErrInvalidControl
		}
		want.Command = op.Command
	case "select_document":
		if !opaqueID(op.Document) {
			return ErrInvalidControl
		}
		want.Document = op.Document
	case "select_text", "insert_text", "replace_selection", "replace_value":
		if !opaqueID(op.Document) || !opaqueID(op.Snapshot) {
			return ErrInvalidControl
		}
		want.Document, want.Snapshot = op.Document, op.Snapshot
		if op.Kind == "select_text" {
			if op.Range == nil || op.Range.Start < 0 || op.Range.End < op.Range.Start {
				return ErrInvalidControl
			}
			want.Range = op.Range
		} else {
			if op.Text == nil || ValidateDocumentInput(*op.Text) != nil {
				return ErrInvalidControl
			}
			want.Text = op.Text
		}
	default:
		return ErrInvalidControl
	}
	if nativeHash(want) != nativeHash(op) {
		return ErrInvalidControl
	}
	return nil
}

func copyHostDocumentOperation(op DocumentOperation) DocumentOperation {
	if op.Range != nil {
		v := *op.Range
		op.Range = &v
	}
	if op.Text != nil {
		v := *op.Text
		op.Text = &v
	}
	return op
}

// NativeDocumentBackend is implemented by an owned native provider. Read must
// not switch tabs or activate controls. Apply methods perform one input attempt,
// after validating the target, composition, selection and focus again. They must
// never fall back to another input method, retry input, or restore old text.
// Implementing this private interface alone does not advertise a wire feature.
type NativeDocumentBackend interface {
	ReadDocument(context.Context, ControlBinding) (DocumentInventory, DocumentSnapshot, error)
	ApplyDocumentText(context.Context, ControlBinding, PreparedDocumentText) error
	ApplyDocumentLifecycle(context.Context, ControlBinding, DocumentInventory, DocumentOperation) error
}

type preparedDocumentAction struct {
	action    PreparedControlAction
	binding   ControlBinding
	inventory DocumentInventory
	snapshot  DocumentSnapshot
	text      *PreparedDocumentText
}

// NativeDocumentExecutor is owned by the existing worker thread. It is not a
// listener, grant authority or alternative journal. Dispatch must be installed
// behind NativeSupervisor so a durable claim precedes every provider call.
type NativeDocumentExecutor struct {
	backend      NativeDocumentBackend
	prepared     map[string]preparedDocumentAction
	measurements *NativeMeasurements
}

// SetMeasurements is called only on the owning worker before executing work.
// Reports remain local unless an explicitly opted-in fixture collects them.
func (e *NativeDocumentExecutor) SetMeasurements(m *NativeMeasurements) { e.measurements = m }

func NewNativeDocumentExecutor(backend NativeDocumentBackend) *NativeDocumentExecutor {
	return &NativeDocumentExecutor{backend: backend, prepared: map[string]preparedDocumentAction{}}
}

func cloneDocumentInventory(in DocumentInventory) DocumentInventory {
	in.Documents = append([]NativeDocument(nil), in.Documents...)
	in.Commands = append([]DocumentCreationCommand(nil), in.Commands...)
	if in.Active != nil {
		b := *in.Active
		in.Active = &b
	}
	return in
}

func (e *NativeDocumentExecutor) Clear() { e.prepared = map[string]preparedDocumentAction{} }

func (e *NativeDocumentExecutor) read(ctx context.Context, binding ControlBinding) (DocumentInventory, DocumentSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return DocumentInventory{}, DocumentSnapshot{}, err
	}
	if binding.Validate() != nil || e.backend == nil {
		return DocumentInventory{}, DocumentSnapshot{}, ErrControlScope
	}
	inventory, snapshot, err := e.backend.ReadDocument(ctx, binding)
	if err != nil {
		return DocumentInventory{}, DocumentSnapshot{}, err
	}
	if err = inventory.Validate(binding.Target, time.Now()); err != nil {
		return DocumentInventory{}, DocumentSnapshot{}, err
	}
	if !inventory.Complete {
		return DocumentInventory{}, DocumentSnapshot{}, ErrDocumentIncomplete
	}
	if inventory.Active != nil {
		if _, err = activeDocument(inventory, snapshot, time.Now()); err != nil {
			return DocumentInventory{}, DocumentSnapshot{}, err
		}
	} else if snapshot.ID != "" {
		return DocumentInventory{}, DocumentSnapshot{}, ErrDocumentChanged
	}
	return cloneDocumentInventory(inventory), copyDocumentSnapshot(snapshot), nil
}

// Read always obtains a fresh native snapshot. It does not return the input
// payload, the prepared preimage, or cached post-dispatch evidence as read-back.
func (e *NativeDocumentExecutor) Read(ctx context.Context, binding ControlBinding) (DocumentInventory, DocumentSnapshot, error) {
	return e.readMeasured(ctx, binding, "observation")
}

func (e *NativeDocumentExecutor) ReadVerification(ctx context.Context, binding ControlBinding) (DocumentInventory, DocumentSnapshot, error) {
	return e.readMeasured(ctx, binding, "verification")
}

func (e *NativeDocumentExecutor) readMeasured(ctx context.Context, binding ControlBinding, stage string) (DocumentInventory, DocumentSnapshot, error) {
	started := time.Now()
	defer func() { e.measurements.Add(stage, time.Since(started)) }()
	return e.read(ctx, binding)
}

// PrepareCurrent handles model/approval latency by doing a real provider read
// before a NEW preparation. It neither extends an old grant nor replays input.
// Caller references are usable only if the complete document set, generation,
// text, selection, capabilities and revision still agree byte-for-byte.
func (e *NativeDocumentExecutor) PrepareCurrent(ctx context.Context, binding ControlBinding, op DocumentOperation, before DocumentInventory, snapshot DocumentSnapshot) (PreparedControlAction, error) {
	started := time.Now()
	defer func() { e.measurements.Add("preparation", time.Since(started)) }()
	if before.Observation.CapturedAt.After(time.Now()) || before.Validate(binding.Target, before.Observation.CapturedAt) != nil {
		return PreparedControlAction{}, ErrStaleObservation
	}
	current, read, err := e.read(ctx, binding)
	if err != nil {
		return PreparedControlAction{}, err
	}
	if !sameDocumentInventory(before, current) {
		return PreparedControlAction{}, ErrDocumentChanged
	}
	if before.Active != nil {
		if err = ValidateDocumentPrecondition(snapshot, read); err != nil {
			return PreparedControlAction{}, err
		}
		// Snapshot identity names the immutable preimage, not its capture time.
		// The fresh binding/observation remains in the sealed host preparation.
		read.ID = snapshot.ID
	}
	return e.Prepare(binding, op, current, read)
}

func (e *NativeDocumentExecutor) Prepare(binding ControlBinding, op DocumentOperation, inventory DocumentInventory, snapshot DocumentSnapshot) (PreparedControlAction, error) {
	if binding.Validate() != nil || op.Validate() != nil || inventory.Validate(binding.Target, time.Now()) != nil {
		return PreparedControlAction{}, ErrInvalidControl
	}
	if !inventory.Complete {
		return PreparedControlAction{}, ErrDocumentIncomplete
	}
	if len(e.prepared) >= MaxStepActions {
		return PreparedControlAction{}, ErrInvalidControl
	}
	if inventory.Active != nil {
		if _, err := activeDocument(inventory, snapshot, time.Now()); err != nil {
			return PreparedControlAction{}, err
		}
		if snapshot.Composition != "stable" {
			return PreparedControlAction{}, ErrDocumentComposition
		}
	}
	op = copyHostDocumentOperation(op)
	preparation := preparedDocumentAction{binding: binding, inventory: cloneDocumentInventory(inventory), snapshot: copyDocumentSnapshot(snapshot)}
	identity, expected := "", nativeHash(op)
	switch op.Kind {
	case "create_document":
		for _, command := range inventory.Commands {
			if command.ID == op.Command {
				identity = command.ID
			}
		}
	case "select_document":
		for _, document := range inventory.Documents {
			if document.ID == op.Document && document.Capabilities.Select {
				identity = document.Container
			}
		}
	default:
		if inventory.Active == nil || inventory.Active.Document != op.Document {
			return PreparedControlAction{}, ErrDocumentChanged
		}
		plan, err := PrepareDocumentText(inventory, snapshot, DocumentTextOperation{Kind: op.Kind, Snapshot: op.Snapshot, Binding: *inventory.Active, Range: op.Range, Text: op.Text}, time.Now())
		if err != nil {
			return PreparedControlAction{}, err
		}
		preparation.text = &plan
		identity, expected = snapshot.Binding.Editor, plan.Digest()
	}
	if identity == "" {
		return PreparedControlAction{}, ErrDocumentUnavailable
	}
	action := PreparedControlAction{ID: nativeID(), Operation: Operation{Kind: "document", Document: &op}, ControlIdentity: identity, Precondition: nativeHash([]any{binding, inventory, snapshot}), ExpectedChange: expected, ApprovalClass: ActionReversible}
	if preparation.text != nil {
		action.ID = preparation.text.ID()
	}
	preparation.action = action
	e.prepared[action.ID] = preparation
	// Do not share mutable pointers with the approval/transport caller.
	copy := copyHostDocumentOperation(op)
	action.Operation.Document = &copy
	return action, nil
}

func sameDocumentInventory(before, after DocumentInventory) bool {
	if before.Observation.Target != after.Observation.Target || !before.Complete || !after.Complete || len(before.Documents) != len(after.Documents) || len(before.Commands) != len(after.Commands) {
		return false
	}
	// Reordering is not replacement. Captions are data, not identity, but any
	// changed capability/selection/command metadata requires new preparation.
	docs := map[string]NativeDocument{}
	for _, d := range before.Documents {
		docs[d.ID] = d
	}
	for _, d := range after.Documents {
		if previous, ok := docs[d.ID]; !ok || previous != d {
			return false
		}
	}
	commands := map[string]DocumentCreationCommand{}
	for _, c := range before.Commands {
		commands[c.ID] = c
	}
	for _, c := range after.Commands {
		if previous, ok := commands[c.ID]; !ok || previous != c {
			return false
		}
	}
	if (before.Active == nil) != (after.Active == nil) {
		return false
	}
	if before.Active != nil {
		a, b := *before.Active, *after.Active
		a.Observation, b.Observation = "", ""
		if a != b {
			return false
		}
	}
	return true
}

func (e *NativeDocumentExecutor) Dispatch(ctx context.Context, action PreparedControlAction) (ControlResult, error) {
	result := ControlResult{ActionID: action.ID, Outcome: "failed"}
	prepared, ok := e.prepared[action.ID]
	if !ok || nativeHash(prepared.action) != nativeHash(action) {
		return result, ErrControlApproval
	}
	// Consume before reading/provider input, even on failure. The journal owns
	// recovery and exactly-once identity; this is never a retryable input token.
	delete(e.prepared, action.ID)
	inventory, snapshot, err := e.read(ctx, prepared.binding)
	if err != nil {
		return result, err
	}
	if !sameDocumentInventory(prepared.inventory, inventory) {
		return result, ErrDocumentChanged
	}
	if inventory.Active != nil && snapshot.Composition != "stable" {
		return result, ErrDocumentComposition
	}
	if prepared.text != nil {
		if err = prepared.text.Revalidate(inventory, snapshot, time.Now()); err != nil {
			return result, err
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	op := *prepared.action.Operation.Document
	// Once a provider is entered a failure can be partial. No error message or
	// elapsed timeout is treated as permission to type/create a second time.
	result.Outcome, result.Uncertain = "uncertain", true
	inputStarted := time.Now()
	if prepared.text != nil {
		err = e.backend.ApplyDocumentText(ctx, prepared.binding, *prepared.text)
	} else {
		err = e.backend.ApplyDocumentLifecycle(ctx, prepared.binding, inventory, op)
	}
	e.measurements.Add("input", time.Since(inputStarted))
	if err != nil {
		return result, ErrUncertain
	}
	verificationStarted := time.Now()
	defer func() { e.measurements.Add("verification", time.Since(verificationStarted)) }()
	deadline := verificationStarted.Add(2 * time.Second)
	for ctx.Err() == nil {
		afterInventory, after, readErr := e.read(ctx, prepared.binding)
		if readErr == nil {
			var evidence DocumentActionEvidence
			switch op.Kind {
			case "create_document":
				evidence, readErr = NewDocumentCreationEvidence(action.ID, op.Command, inventory, afterInventory, after, time.Now())
			case "select_document":
				readErr = ErrDocumentChanged
				if afterInventory.Active != nil && afterInventory.Active.Document == op.Document && preservedDocumentSet(prepared.inventory, afterInventory) && after.Composition == "stable" {
					for _, d := range prepared.inventory.Documents {
						if d.ID == op.Document && d.Generation == afterInventory.Active.Generation && d.Container == afterInventory.Active.Container {
							readErr = nil
						}
					}
				}
			case "select_text":
				readErr = ErrDocumentChanged
				if after.Complete && SameDocument(prepared.snapshot.Binding, after.Binding) && after.Text == prepared.snapshot.Text && after.Composition == "stable" && after.Selection != nil && *after.Selection == prepared.text.ExpectedSelection() {
					readErr = nil
				}
			default:
				evidence, readErr = NewDocumentEditEvidence(*prepared.text, afterInventory, after, time.Now())
			}
			if readErr == nil && afterInventory.Observation.ID != inventory.Observation.ID {
				result.Outcome, result.Uncertain = "verified", false
				result.EvidenceID = nativeHash([]any{action.ID, afterInventory.Active, after.Selection})
				if evidence.ActionID != "" {
					result.Document = &evidence
				}
				return result, nil
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return result, ErrUncertain
}

func preservedDocumentSet(before, after DocumentInventory) bool {
	if !before.Complete || !after.Complete || len(before.Documents) != len(after.Documents) {
		return false
	}
	for _, old := range before.Documents {
		found := false
		for _, current := range after.Documents {
			if old.ID == current.ID && old.Generation == current.Generation && old.Container == current.Container {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
