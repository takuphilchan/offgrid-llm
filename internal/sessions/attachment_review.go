package sessions

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// AttachmentReviewText is for source inspection only. Failed OCR remains failed
// and cannot enter AttachmentSources or generation through this method.
func (s *ScopedManager) AttachmentReviewText(name, id string) (Attachment, string, string, error) {
	a, text, parser, _, err := s.AttachmentReviewDocument(name, id)
	return a, text, parser, err
}

// Failed extraction graphs are inspectable only here; ordinary source admission
// still requires a ready attachment and its verified revision.
func (s *ScopedManager) AttachmentReviewDocument(name, id string, revision ...string) (Attachment, string, string, *document.Extraction, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, "", "", nil, err
	}
	defer root.Close()
	pinned := ""
	if len(revision) > 0 {
		pinned = revision[0]
	}
	record, err := readAttachmentRecordRevision(root, dir, id, pinned)
	if err != nil {
		return Attachment{}, "", "", nil, err
	}
	if err = verifyAttachmentOriginal(root, dir, record); err != nil {
		return Attachment{}, "", "", nil, err
	}
	if record.Status == "ready" {
		digest := sha256.Sum256([]byte(record.SHA256 + "\x00" + record.Parser + "\x00" + record.Text))
		if hex.EncodeToString(digest[:]) != record.Revision {
			return Attachment{}, "", "", nil, ErrAttachmentUnavailable
		}
	}
	graph, err := readAttachmentStructure(root, dir, record)
	if err != nil {
		return Attachment{}, "", "", nil, err
	}
	return record.Attachment, record.Text, record.Parser, graph, nil
}
