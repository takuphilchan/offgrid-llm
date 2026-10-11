package computer

import "encoding/json"

// A negotiated schema revision is not authority or platform qualification.
// Surface-level capabilities must additionally permit each requested operation.
type NativeFeatures struct {
	Documents int `json:"native_documents,omitempty"`
}

func (f *NativeFeatures) UnmarshalJSON(data []byte) error {
	var wire struct {
		Documents json.RawMessage `json:"native_documents,omitempty"`
	}
	if DecodeControlMessage(data, &wire) != nil {
		return ErrInvalidControl
	}
	*f = NativeFeatures{}
	if wire.Documents != nil {
		if json.Unmarshal(wire.Documents, &f.Documents) != nil || f.Documents != NativeDocumentsVersion {
			return ErrInvalidControl
		}
	}
	return nil
}

func (f NativeFeatures) Validate() error {
	if f.Documents != 0 && f.Documents != NativeDocumentsVersion {
		return ErrInvalidControl
	}
	return nil
}

// Providers must implement both the contract and the owned execution interface.
// Edit support is separately reported by each current document. A read-only
// provider may reject every apply call; a version flag alone is not readiness.
type NativeDocumentProvider interface {
	NativeDocumentBackend
	DocumentContractVersion() int
}

func ProviderNativeFeatures(provider NativeAdapter) NativeFeatures {
	if documents, ok := provider.(NativeDocumentProvider); ok && documents.DocumentContractVersion() == NativeDocumentsVersion {
		return NativeFeatures{Documents: NativeDocumentsVersion}
	}
	return NativeFeatures{}
}

func (h *BrowserHub) NativeFeatures(actor, id string) (NativeFeatures, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return NativeFeatures{}, err
	}
	return s.NativeFeatures, nil
}

func (h *BrowserHub) PairNativeFeatures(code, title string, target TargetIdentity, protocol int, mode ApprovalMode, features NativeFeatures) (*BrowserSession, string, error) {
	if features.Validate() != nil || protocol != ControlProtocolVersion || !target.valid() || target.Driver == "browser" || len(title) == 0 || len(title) > 1024 {
		return nil, "", ErrControlScope
	}
	return h.pair(code, title, target.Driver, &target, mode, false, features)
}

func (h *BrowserHub) PairDesktopFeatures(code string, target TargetIdentity, mode ApprovalMode, features NativeFeatures) (*BrowserSession, string, error) {
	if features.Validate() != nil || target.Validate() != nil || target.Driver == "browser" || target.Surface != "desktop" || target.ProcessGeneration != target.ID {
		return nil, "", ErrControlScope
	}
	return h.pair(code, "Task desktop", target.Driver, &target, mode, true, features)
}
