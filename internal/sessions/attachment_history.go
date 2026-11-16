package sessions

import "time"

type AttachmentRevision struct {
	Revision           string     `json:"revision"`
	ExtractionRevision string     `json:"extraction_revision"`
	ParentRevision     string     `json:"parent_revision,omitempty"`
	Author             string     `json:"author,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
}

type AttachmentRevisionHistory struct {
	ID              string               `json:"id"`
	CurrentRevision string               `json:"current_revision"`
	Revisions       []AttachmentRevision `json:"revisions"`
}

// At most 65 small metadata records, newest first, under current source access.
func (s *ScopedManager) AttachmentHistory(name, id string) (*AttachmentRevisionHistory, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	current, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return nil, err
	}
	if err := verifyAttachmentOriginal(root, dir, current); err != nil {
		return nil, err
	}
	history := &AttachmentRevisionHistory{ID: id, CurrentRevision: current.Revision, Revisions: []AttachmentRevision{}}
	revisions := append(append([]string{}, current.PreviousRevisions...), current.Revision)
	for i := len(revisions) - 1; i >= 0; i-- {
		r, err := readPublishedAttachmentRevision(root, dir, current, revisions[i])
		if err != nil {
			return nil, err
		}
		history.Revisions = append(history.Revisions, AttachmentRevision{Revision: r.Revision, ExtractionRevision: r.ExtractionRevision, ParentRevision: r.ParentRevision, Author: r.CorrectionAuthor, CreatedAt: r.CorrectedAt})
	}
	return history, nil
}
