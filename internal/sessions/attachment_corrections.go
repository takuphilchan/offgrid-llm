package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

var ErrAttachmentCorrectionInvalid = errors.New("invalid attachment correction")

// Caller ownership is checked independently of administrative read access.
func (s *ScopedManager) canCorrectAttachment(session *Session) bool {
	return session.OwnerID == s.access.UserID && (s.access.UserID != "" || s.access.All)
}

func (s *ScopedManager) CorrectAttachment(ctx context.Context, name, id, expected, extraction string, edits []document.Correction) (Attachment, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return Attachment{}, err
	}
	if err := s.checkCanonicalName(name); err != nil {
		return Attachment{}, err
	}
	session, err := s.Load(name)
	if err != nil {
		return Attachment{}, err
	}
	if !s.canCorrectAttachment(session) {
		return Attachment{}, ErrAccessDenied
	}
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, err
	}
	defer root.Close()
	current, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return Attachment{}, err
	}
	if current.Status != "ready" || current.ExtractionRevision == "" {
		return Attachment{}, ErrAttachmentUnavailable
	}
	if expected == "" || extraction == "" || current.Revision != expected || current.ExtractionRevision != extraction {
		return Attachment{}, ErrTurnConflict
	}
	if len(current.PreviousRevisions) >= maxAttachmentRevisions {
		return Attachment{}, ErrAttachmentLimit
	}
	if document.TextSHA256(current.SHA256+"\x00"+current.Parser+"\x00"+current.Text) != current.Revision || verifyAttachmentOriginal(root, dir, current) != nil {
		return Attachment{}, ErrAttachmentUnavailable
	}
	parent, err := readAttachmentStructure(root, dir, current)
	if err != nil || parent == nil {
		return Attachment{}, ErrAttachmentUnavailable
	}
	author := s.access.UserID
	if author == "" {
		author = "local"
	}
	derived, err := parent.Correct(author, time.Now().UTC(), edits)
	if err != nil || strings.TrimSpace(derived.Text) == "" {
		return Attachment{}, ErrAttachmentCorrectionInvalid
	}
	next := current
	next.PreviousRevisions = append(append([]string{}, current.PreviousRevisions...), current.Revision)
	next.ParentRevision = current.Revision
	next.ExtractionRevision = derived.ID
	next.CorrectionAuthor = author
	next.CorrectedAt = derived.CreatedAt
	next.Text = derived.Text
	next.Parser = "document-review-v1/" + derived.ID + "/history-" + attachmentHistoryDigest(next.PreviousRevisions)
	if next.Evidence != nil {
		data, err := json.Marshal(next.Evidence)
		if err != nil {
			return Attachment{}, err
		}
		next.Parser += "/coverage-" + document.TextSHA256(string(data))[:16]
	}
	next.Revision = document.TextSHA256(next.SHA256 + "\x00" + next.Parser + "\x00" + next.Text)
	if err = s.writeAttachmentRevision(root, dir, name, current, parent); err != nil {
		return Attachment{}, err
	}
	if err = s.writeAttachmentRevision(root, dir, name, next, derived); err != nil {
		return Attachment{}, err
	}
	if err = ctx.Err(); err != nil {
		return Attachment{}, err
	}
	// Both artifacts are durable before the sole publication point. Old readers
	// retain their pinned source; drafts are never silently moved to this revision.
	if err = writeAttachmentRecord(root, dir, next); err != nil {
		return Attachment{}, err
	}
	return next.Attachment, nil
}
