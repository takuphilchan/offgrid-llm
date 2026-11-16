package sessions

import (
	"errors"
	"os"
	"strings"
)

const maxActorAttachmentBytes int64 = 500 << 20

// Caller holds mutationMu. Count originals or derived graphs, including
// interrupted writes, across this owner's conversations. Admin access does not
// merge owner quotas; originals and structures use independent byte limits.
func (s *ScopedManager) checkAttachmentActorQuota(name string, incoming, limit int64, suffixes ...string) error {
	suffix := ".bin"
	if len(suffixes) > 0 {
		suffix = suffixes[0]
	}
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
			if !strings.HasSuffix(entry.Name(), suffix) && !(suffix == ".revision-" && strings.Contains(entry.Name(), ".revision-")) && !(suffix == ".ocr-" && strings.Contains(entry.Name(), ".ocr-")) && !(suffix == ".structure" && strings.Contains(entry.Name(), ".structure.") && strings.HasSuffix(entry.Name(), ".partial")) {
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
