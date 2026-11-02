package sessions

import (
	"errors"
	"fmt"
	"os"
)

// Caller holds mutationMu. The hash-derived directory is checked by os.Root;
// removal never follows links or recurses into an unexpected directory. A crash
// leaves the conversation in place for explicit retry, not a successful delete.
func (sm *SessionManager) removeConversationFiles(name string) error {
	scope := sm.WithAccess(Access{All: true})
	root, dir, err := scope.attachmentRoot(name, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.Open(dir)
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("attachment cleanup found an unexpected directory")
		}
		if err := root.Remove(dir + "/" + entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := root.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Removing a draft chip does not delete a source used in history. Source deletion
// is explicit and cannot invalidate a completed or in-flight conversation turn.
func (s *ScopedManager) DeleteAttachment(name, id string) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return err
	}
	if !attachmentID.MatchString(id) {
		return ErrAttachmentUnavailable
	}
	if session.Turn.Active() {
		return ErrAttachmentInUse
	}
	for _, message := range session.Messages {
		for _, ref := range message.Attachments {
			if ref.ID == id {
				return ErrAttachmentInUse
			}
		}
	}
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return err
	}
	defer root.Close()
	metadata := dir + "/" + id + ".json"
	tombstone := dir + "/" + id + ".removed"
	if _, err := root.Lstat(tombstone); errors.Is(err, os.ErrNotExist) {
		if _, err := readAttachmentRecord(root, dir, id); err != nil {
			return err
		}
		if err := root.Rename(metadata, tombstone); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := root.Remove(dir + "/" + id + ".bin"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return root.Remove(tombstone)
}
