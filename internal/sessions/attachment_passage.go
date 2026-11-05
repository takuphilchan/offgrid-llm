package sessions

import "unicode/utf8"

// AttachmentPassage is an exact slice of the pinned extraction, not a fresh
// parse, model interpretation, or an assertion that a claim is supported.
type AttachmentPassage struct {
	ID        string `json:"id"`
	Revision  string `json:"revision"`
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text"`
}

func (s *ScopedManager) ReadAttachmentPassage(name string, ref AttachmentReference, start, end int) (AttachmentPassage, error) {
	if start < 0 || end <= start || end-start > MaxAttachmentContextBytes {
		return AttachmentPassage{}, ErrAttachmentUnavailable
	}
	sources, err := s.AttachmentSources(name, []AttachmentReference{ref})
	if err != nil {
		return AttachmentPassage{}, err
	}
	source := sources[0]
	if end > len(source.Text) || !utf8.ValidString(source.Text[start:end]) {
		return AttachmentPassage{}, ErrAttachmentUnavailable
	}
	return AttachmentPassage{ID: ref.ID, Revision: ref.Revision, Name: source.Attachment.Name, SHA256: source.Attachment.SHA256, StartByte: start, EndByte: end, Text: source.Text[start:end]}, nil
}
