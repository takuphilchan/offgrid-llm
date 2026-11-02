package sessions

import (
	"errors"
	"os"
	"strings"
)

const maxActorAttachmentBytes int64 = 500 << 20

// Caller holds mutationMu. Count original bytes, including interrupted uploads,
// across this owner’s conversations. Admin access does not merge owner quotas.
func (s *ScopedManager) checkAttachmentActorQuota(name string, incoming, limit int64) error {
	target, err := s.Load(name)
	if err != nil {
		return err
	}
	inventory, err := s.manager.List()
	if err != nil {
		return err
	}
	trusted := s.manager.WithAccess(Access{All: true})
	total := incoming
	for _, session := range inventory {
		if session.OwnerID != target.OwnerID {
			continue
		}
		root, dir, err := trusted.attachmentRoot(session.Name, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		f, err := root.Open(dir)
		if err != nil {
			root.Close()
			return err
		}
		entries, err := f.ReadDir(-1)
		f.Close()
		if err != nil {
			root.Close()
			return err
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".bin") {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				root.Close()
				return ErrAttachmentUnavailable
			}
			if info.Size() > limit-total {
				root.Close()
				return ErrAttachmentLimit
			}
			total += info.Size()
		}
		root.Close()
	}
	if total > limit {
		return ErrAttachmentLimit
	}
	return nil
}
