package computer

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Contracts alone do not advertise a capability. A provider and the private
// transport must negotiate this revision before any document tool is exposed.
const NativeDocumentsVersion = 1
const MaxDocumentInputBytes = 16000
const MaxDocumentSnapshotBytes = 64 << 10
const MaxDocumentPageBytes = 4 << 10
const MaxDocumentInventory = 256

var (
	ErrDocumentUnavailable = errors.New("computer_document_unavailable")
	ErrDocumentIncomplete  = errors.New("computer_document_incomplete")
	ErrDocumentRange       = errors.New("computer_document_range_invalid")
	ErrDocumentComposition = errors.New("computer_document_composition_unstable")
)

type DocumentCapabilities struct {
	Read             bool `json:"read"`
	Select           bool `json:"select"`
	SelectText       bool `json:"select_text"`
	InsertText       bool `json:"insert_text"`
	ReplaceSelection bool `json:"replace_selection"`
	ReplaceValue     bool `json:"replace_value"`
}

// Identity is not a caption, array index, window or editor runtime ID. Providers
// must rotate Generation on document replacement or unknown continuity.
type NativeDocument struct {
	ID           string               `json:"id"`
	Generation   string               `json:"generation"`
	Container    string               `json:"container"`
	Title        string               `json:"title"`
	Selected     bool                 `json:"selected"`
	Capabilities DocumentCapabilities `json:"capabilities"`
}

type DocumentBinding struct {
	Target      TargetIdentity `json:"target"`
	Document    string         `json:"document"`
	Generation  string         `json:"generation"`
	Container   string         `json:"container"`
	Editor      string         `json:"editor"`
	Observation string         `json:"observation"`
	Revision    string         `json:"revision"`
}

func (b DocumentBinding) Validate() error {
	if b.Target.Validate() != nil || b.Target.Driver == "browser" || !opaqueID(b.Document) || !opaqueID(b.Generation) || !opaqueID(b.Container) || !opaqueID(b.Editor) || !opaqueID(b.Observation) || !opaqueID(b.Revision) {
		return ErrDocumentUnavailable
	}
	return nil
}

type DocumentCreationCommand struct {
	ID       string `json:"id"`
	Element  string `json:"element"`
	Kind     string `json:"kind"` // closed new_document or new_tab semantics, not a key macro
	Profile  string `json:"profile"`
	Revision string `json:"revision"`
}

type DocumentInventory struct {
	Version     int                       `json:"version"`
	Observation ControlObservation        `json:"observation"`
	Documents   []NativeDocument          `json:"documents"`
	Active      *DocumentBinding          `json:"active,omitempty"`
	Complete    bool                      `json:"complete"`
	Commands    []DocumentCreationCommand `json:"commands"`
}

func (v DocumentInventory) Validate(target TargetIdentity, now time.Time) error {
	if v.Version != NativeDocumentsVersion || len(v.Documents) > MaxDocumentInventory || len(v.Commands) > 16 {
		return ErrDocumentUnavailable
	}
	if err := v.Observation.Validate(target, now); err != nil {
		return err
	}
	ids, containers := map[string]bool{}, map[string]bool{}
	selected := 0
	var active NativeDocument
	for _, doc := range v.Documents {
		if !opaqueID(doc.ID) || !opaqueID(doc.Generation) || !opaqueID(doc.Container) || len(doc.Title) > 1024 || !utf8.ValidString(doc.Title) || strings.ContainsRune(doc.Title, 0) || ids[doc.ID] || containers[doc.Container] {
			return ErrDocumentUnavailable
		}
		ids[doc.ID], containers[doc.Container] = true, true
		if doc.Selected {
			selected++
			active = doc
		}
		if (doc.Capabilities.SelectText || doc.Capabilities.InsertText || doc.Capabilities.ReplaceSelection || doc.Capabilities.ReplaceValue) && !doc.Capabilities.Read {
			return ErrDocumentUnavailable
		}
	}
	if selected > 1 {
		return ErrDocumentUnavailable
	}
	if v.Active != nil {
		b := v.Active
		if b.Validate() != nil || selected != 1 || b.Target != target || b.Observation != v.Observation.ID || b.Document != active.ID || b.Generation != active.Generation || b.Container != active.Container {
			return ErrDocumentUnavailable
		}
	} else if v.Complete && selected != 0 {
		return ErrDocumentIncomplete
	}
	commands := map[string]bool{}
	for _, command := range v.Commands {
		if !opaqueID(command.ID) || !opaqueID(command.Element) || !opaqueID(command.Profile) || !opaqueID(command.Revision) || commands[command.ID] || (command.Kind != "new_document" && command.Kind != "new_tab") {
			return ErrDocumentUnavailable
		}
		commands[command.ID] = true
	}
	return nil
}

// Creation requires a complete before/after inventory and a new identity, not a
// newly selected old tab, a renamed title, or a reused document/editor.
func ValidateCreatedDocument(before, after DocumentInventory, text DocumentSnapshot, now time.Time) error {
	if before.Validate(before.Observation.Target, now) != nil || after.Validate(before.Observation.Target, now) != nil || !before.Complete || !after.Complete || after.Active == nil {
		return ErrDocumentIncomplete
	}
	if after.Active.Observation == before.Observation.ID {
		return ErrStaleObservation
	}
	if len(after.Documents) != len(before.Documents)+1 {
		return ErrDocumentChanged
	}
	for _, doc := range before.Documents {
		if doc.ID == after.Active.Document || doc.Container == after.Active.Container {
			return ErrDocumentChanged
		}
		preserved := false
		for _, current := range after.Documents {
			if current.ID == doc.ID && current.Generation == doc.Generation && current.Container == doc.Container {
				preserved = true
				break
			}
		}
		if !preserved {
			return ErrDocumentChanged
		}
	}
	if text.Validate() != nil || !text.Complete || text.Binding != *after.Active || text.Text != "" || text.Composition != "stable" {
		return ErrDocumentIncomplete
	}
	return nil
}

// A half-open range of Unicode scalar offsets over one complete snapshot.
// Byte, UTF-16 and provider-native offsets are never model-visible ranges.
type DocumentTextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Full bounded snapshots are host state, not a promise to copy 64 KiB into every
// model request. Transport projections use revision-bound pages of at most 4 KiB.
type DocumentSnapshot struct {
	ID          string             `json:"id"`
	Binding     DocumentBinding    `json:"binding"`
	Text        string             `json:"text"`
	Complete    bool               `json:"complete"`
	Selection   *DocumentTextRange `json:"selection,omitempty"`
	Composition string             `json:"composition"` // stable, active, unknown
}

func (s DocumentSnapshot) Validate() error {
	if !opaqueID(s.ID) || s.Binding.Validate() != nil || !utf8.ValidString(s.Text) || strings.ContainsRune(s.Text, 0) || len(s.Text) > MaxDocumentSnapshotBytes {
		return ErrDocumentUnavailable
	}
	if s.Composition != "stable" && s.Composition != "active" && s.Composition != "unknown" {
		return ErrDocumentUnavailable
	}
	if s.Selection != nil {
		if !s.Complete {
			return ErrDocumentIncomplete
		}
		if _, err := DocumentRangeToNative(s.Text, *s.Selection, TextUnitScalar); err != nil {
			return err
		}
	}
	return nil
}

func ValidateDocumentInput(text string) error {
	if len(text) > MaxDocumentInputBytes || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return ErrInvalidControl
	}
	return nil
}

// This proves unchanged preparation inputs, not the result of a mutation. Each
// native provider must independently read back the post-dispatch document.
func ValidateDocumentPrecondition(expected, current DocumentSnapshot) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if !expected.Complete || !current.Complete {
		return ErrDocumentIncomplete
	}
	if expected.Composition != "stable" || current.Composition != "stable" {
		return ErrDocumentComposition
	}
	before, after := expected.Binding, current.Binding
	before.Observation, after.Observation = "", "" // a new read may have a new observation, never a new document/revision
	if before != after || expected.Text != current.Text || expected.Selection == nil || current.Selection == nil || *expected.Selection != *current.Selection {
		return ErrDocumentChanged
	}
	return nil
}
