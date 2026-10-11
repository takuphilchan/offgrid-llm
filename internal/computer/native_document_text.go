package computer

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

// DocumentTextOperation is not an alternative dispatch API. The native host
// prepares this union after authorization; the existing journal must claim the
// resulting action before any provider input. Unknown/mixed modes fail closed.
type DocumentTextOperation struct {
	Kind     string             `json:"kind"`
	Snapshot string             `json:"snapshot"`
	Binding  DocumentBinding    `json:"binding"`
	Range    *DocumentTextRange `json:"range,omitempty"`
	Text     *string            `json:"text,omitempty"`
}

func (op DocumentTextOperation) Validate() error {
	if !opaqueID(op.Snapshot) || op.Binding.Validate() != nil {
		return ErrInvalidControl
	}
	switch op.Kind {
	case "select_text":
		if op.Text != nil || op.Range == nil || op.Range.Start < 0 || op.Range.End < op.Range.Start {
			return ErrInvalidControl
		}
	case "insert_text", "replace_selection", "replace_value":
		if op.Range != nil || op.Text == nil || ValidateDocumentInput(*op.Text) != nil {
			return ErrInvalidControl
		}
	default:
		return ErrInvalidControl
	}
	return nil
}

// PreparedDocumentText contains private host state. It is intentionally not a
// wire structure: preimages must not appear in general logs or model context.
// Copies prevent a caller changing a range/text pointer after preparation.
type PreparedDocumentText struct {
	id           string
	operation    DocumentTextOperation
	before       DocumentSnapshot
	capabilities DocumentCapabilities
	expected     string
	selection    DocumentTextRange
}

func copyDocumentSnapshot(s DocumentSnapshot) DocumentSnapshot {
	if s.Selection != nil {
		r := *s.Selection
		s.Selection = &r
	}
	return s
}
func copyDocumentOperation(op DocumentTextOperation) DocumentTextOperation {
	if op.Range != nil {
		r := *op.Range
		op.Range = &r
	}
	if op.Text != nil {
		text := *op.Text
		op.Text = &text
	}
	return op
}

func activeDocument(inventory DocumentInventory, snapshot DocumentSnapshot, now time.Time) (NativeDocument, error) {
	if err := snapshot.Validate(); err != nil {
		return NativeDocument{}, err
	}
	if err := inventory.Validate(snapshot.Binding.Target, now); err != nil {
		return NativeDocument{}, err
	}
	if !inventory.Complete || !snapshot.Complete || inventory.Active == nil {
		return NativeDocument{}, ErrDocumentIncomplete
	}
	if *inventory.Active != snapshot.Binding {
		return NativeDocument{}, ErrDocumentChanged
	}
	for _, document := range inventory.Documents {
		if document.Selected && document.ID == snapshot.Binding.Document {
			return document, nil
		}
	}
	return NativeDocument{}, ErrDocumentUnavailable
}

func PrepareDocumentText(inventory DocumentInventory, snapshot DocumentSnapshot, op DocumentTextOperation, now time.Time) (PreparedDocumentText, error) {
	if op.Validate() != nil {
		return PreparedDocumentText{}, ErrInvalidControl
	}
	document, err := activeDocument(inventory, snapshot, now)
	if err != nil {
		return PreparedDocumentText{}, err
	}
	if snapshot.ID != op.Snapshot || snapshot.Binding != op.Binding {
		return PreparedDocumentText{}, ErrDocumentChanged
	}
	if err := ValidateDocumentPrecondition(snapshot, snapshot); err != nil {
		return PreparedDocumentText{}, err
	}
	caps := document.Capabilities
	selection := *snapshot.Selection
	expected := snapshot.Text
	switch op.Kind {
	case "select_text":
		if !caps.SelectText {
			return PreparedDocumentText{}, ErrDocumentUnavailable
		}
		if _, err := DocumentRangeToNative(snapshot.Text, *op.Range, TextUnitScalar); err != nil {
			return PreparedDocumentText{}, err
		}
		selection = *op.Range
	case "insert_text":
		if !caps.InsertText {
			return PreparedDocumentText{}, ErrDocumentUnavailable
		}
		if selection.Start != selection.End {
			return PreparedDocumentText{}, ErrDocumentRange
		}
	case "replace_selection":
		if !caps.ReplaceSelection {
			return PreparedDocumentText{}, ErrDocumentUnavailable
		}
		if selection.Start == selection.End {
			return PreparedDocumentText{}, ErrDocumentRange
		}
	case "replace_value":
		if !caps.ReplaceValue {
			return PreparedDocumentText{}, ErrDocumentUnavailable
		}
		selection = DocumentTextRange{End: utf8.RuneCountInString(snapshot.Text)}
	}
	if op.Text != nil {
		bytes, err := DocumentRangeToNative(snapshot.Text, selection, TextUnitUTF8)
		if err != nil {
			return PreparedDocumentText{}, err
		}
		if len(snapshot.Text)-(bytes.End-bytes.Start)+len(*op.Text) > MaxDocumentSnapshotBytes {
			return PreparedDocumentText{}, ErrDocumentIncomplete
		}
		expected = snapshot.Text[:bytes.Start] + *op.Text + snapshot.Text[bytes.End:]
		caret := selection.Start + utf8.RuneCountInString(*op.Text)
		selection = DocumentTextRange{Start: caret, End: caret}
		// Inserting a combining/ZWJ sequence can join a following cluster. Do not
		// prepare a caret that the shared contract could not later represent.
		if _, err := DocumentRangeToNative(expected, selection, TextUnitScalar); err != nil {
			return PreparedDocumentText{}, err
		}
	}
	return PreparedDocumentText{id: nativeID(), operation: copyDocumentOperation(op), before: copyDocumentSnapshot(snapshot), capabilities: caps, expected: expected, selection: selection}, nil
}

func (p PreparedDocumentText) ID() string { return p.id }
func (p PreparedDocumentText) Operation() DocumentTextOperation {
	return copyDocumentOperation(p.operation)
}
func (p PreparedDocumentText) Preimage() DocumentSnapshot           { return copyDocumentSnapshot(p.before) }
func (p PreparedDocumentText) ExpectedText() string                 { return p.expected }
func (p PreparedDocumentText) ExpectedSelection() DocumentTextRange { return p.selection }
func (p PreparedDocumentText) Digest() string {
	return nativeHash([]any{NativeDocumentsVersion, p.id, p.operation, p.before, p.capabilities, p.expected, p.selection})
}

// Revalidation permits a fresh observation ID but not changed content, document,
// selection, generation, capabilities or revision. It never performs input.
func (p PreparedDocumentText) Revalidate(inventory DocumentInventory, current DocumentSnapshot, now time.Time) error {
	if p.id == "" {
		return ErrInvalidControl
	}
	document, err := activeDocument(inventory, current, now)
	if err != nil {
		return err
	}
	if document.Capabilities != p.capabilities {
		return ErrDocumentChanged
	}
	return ValidateDocumentPrecondition(p.before, current)
}

type DocumentTextPage struct {
	Snapshot       string          `json:"snapshot"`
	Binding        DocumentBinding `json:"binding"`
	Start          int             `json:"start"`
	End            int             `json:"end"`
	TotalScalars   int             `json:"total_scalars"`
	Text           string          `json:"text"`
	SourceComplete bool            `json:"source_complete"`
	Next           *int            `json:"next,omitempty"`
}

type DocumentPageRequest struct {
	Snapshot string `json:"snapshot"`
	Revision string `json:"revision"`
	Start    int    `json:"start"`
}

func (p DocumentPageRequest) Validate() error {
	if !opaqueID(p.Snapshot) || !opaqueID(p.Revision) || p.Start < 0 || p.Start > MaxDocumentSnapshotBytes {
		return ErrDocumentRange
	}
	return nil
}

func (p DocumentTextPage) Validate() error {
	if !opaqueID(p.Snapshot) || p.Binding.Validate() != nil || !p.SourceComplete || !utf8.ValidString(p.Text) || strings.ContainsRune(p.Text, 0) || len(p.Text) > MaxDocumentPageBytes || p.Start < 0 || p.End < p.Start || p.End > p.TotalScalars || p.TotalScalars > MaxDocumentSnapshotBytes || utf8.RuneCountInString(p.Text) != p.End-p.Start {
		return ErrDocumentIncomplete
	}
	if p.End < p.TotalScalars {
		if p.Next == nil || *p.Next != p.End || p.End == p.Start {
			return ErrDocumentIncomplete
		}
	} else if p.Next != nil {
		return ErrDocumentIncomplete
	}
	return nil
}

// DocumentPager owns one immutable complete snapshot. Its caller must obtain a
// fresh authenticated provider snapshot for each page; a client cannot supply
// that current snapshot or make a stale revision readable by echoing its ID.
type DocumentPager struct{ snapshot DocumentSnapshot }

func NewDocumentPager(snapshot DocumentSnapshot) (DocumentPager, error) {
	if err := snapshot.Validate(); err != nil {
		return DocumentPager{}, err
	}
	if !snapshot.Complete {
		return DocumentPager{}, ErrDocumentIncomplete
	}
	return DocumentPager{snapshot: copyDocumentSnapshot(snapshot)}, nil
}

func (p DocumentPager) Page(snapshotID, revision string, start int, current DocumentSnapshot) (DocumentTextPage, error) {
	before := p.snapshot
	if before.ID == "" || before.ID != snapshotID || before.Binding.Revision != revision {
		return DocumentTextPage{}, ErrStaleObservation
	}
	if err := current.Validate(); err != nil {
		return DocumentTextPage{}, err
	}
	if !current.Complete {
		return DocumentTextPage{}, ErrDocumentIncomplete
	}
	expectedBinding, currentBinding := before.Binding, current.Binding
	expectedBinding.Observation, currentBinding.Observation = "", ""
	if expectedBinding != currentBinding || before.Text != current.Text {
		return DocumentTextPage{}, ErrDocumentChanged
	}
	if _, err := DocumentRangeToNative(before.Text, DocumentTextRange{Start: start, End: start}, TextUnitScalar); err != nil {
		return DocumentTextPage{}, err
	}
	page := DocumentTextPage{Snapshot: before.ID, Binding: before.Binding, Start: start, End: start, TotalScalars: utf8.RuneCountInString(before.Text), SourceComplete: true}
	clusters := graphemes.FromString(before.Text)
	scalar, byteStart, byteEnd := 0, 0, 0
	for clusters.Next() {
		cluster := clusters.Value()
		count := utf8.RuneCountInString(cluster)
		if scalar < start {
			byteStart += len(cluster)
			byteEnd = byteStart
			scalar += count
			continue
		}
		if byteEnd-byteStart+len(cluster) > MaxDocumentPageBytes {
			if byteEnd == byteStart {
				return DocumentTextPage{}, ErrDocumentRange
			}
			break
		}
		byteEnd += len(cluster)
		scalar += count
		page.End = scalar
	}
	page.Text = before.Text[byteStart:byteEnd]
	if page.End < page.TotalScalars {
		next := page.End
		page.Next = &next
	}
	return page, nil
}
