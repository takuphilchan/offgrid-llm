package computer

import "time"

// Native adapters own their accessibility references. This state maps those
// references to task-local document identities; it never searches by a caption.
type nativeDocumentState struct {
	target TargetIdentity
	docs   map[string]nativeDocumentIdentity
}

type nativeDocumentIdentity struct{ id, generation, editor, revision, textHash string }

func (s *nativeDocumentState) inventory(target TargetIdentity, topology documentTopology, text string, selection *DocumentTextRange, composition string, caps DocumentCapabilities, commands []DocumentCreationCommand, sequence uint64) (DocumentInventory, DocumentSnapshot, *nativeDocumentState, error) {
	if s == nil || s.target != target {
		s = &nativeDocumentState{target: target, docs: map[string]nativeDocumentIdentity{}}
	}
	next := &nativeDocumentState{target: target, docs: map[string]nativeDocumentIdentity{}}
	observation := ControlObservation{ID: nativeID(), Target: target, Sequence: sequence, CapturedAt: time.Now()}
	inv := DocumentInventory{Version: NativeDocumentsVersion, Observation: observation, Complete: true, Documents: []NativeDocument{}, Commands: append([]DocumentCreationCommand{}, commands...)}
	var snap DocumentSnapshot
	for _, tab := range topology.tabs {
		identity, exists := s.docs[tab.key]
		if !exists {
			identity = nativeDocumentIdentity{id: nativeID(), generation: nativeID()}
		}
		capabilities := DocumentCapabilities{Select: tab.selectable}
		if tab.key == topology.active {
			if identity.editor != "" && identity.editor != topology.editor {
				identity.generation = nativeID()
			}
			identity.editor = topology.editor
			digest := DocumentTextDigest(text)
			if identity.revision == "" || identity.textHash != digest {
				identity.revision, identity.textHash = nativeID(), digest
			}
			bound := DocumentBinding{Target: target, Document: identity.id, Generation: identity.generation, Container: tab.key, Editor: topology.editor, Observation: observation.ID, Revision: identity.revision}
			inv.Active = &bound
			snap = DocumentSnapshot{ID: nativeID(), Binding: bound, Text: text, Complete: true, Selection: selection, Composition: composition}
			capabilities = caps
			capabilities.Select = tab.selectable
		}
		next.docs[tab.key] = identity
		inv.Documents = append(inv.Documents, NativeDocument{ID: identity.id, Generation: identity.generation, Container: tab.key, Title: tab.title, Selected: tab.selected, Capabilities: capabilities})
	}
	inv.Observation.StateDigest = nativeHash([]any{inv.Documents, inv.Commands, inv.Active, DocumentTextDigest(text), selection, composition})
	if inv.Validate(target, time.Now()) != nil || snap.Validate() != nil {
		return DocumentInventory{}, DocumentSnapshot{}, nil, ErrDocumentIncomplete
	}
	return inv, snap, next, nil
}
