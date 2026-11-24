package execution

import (
	"context"
	"errors"
	"io"
	"os"
)

// Release is a trusted supervisor operation after sessions/processes settle.
// It never adopts a recreated pathname or dispatches itself during restart.
func (s *WorkingCopies) Release(ctx context.Context, id string, owner Binding) (resultErr error) {
	return s.releaseCopy(ctx, id, owner, nil)
}

func (s *WorkingCopies) releaseCopy(ctx context.Context, id string, owner Binding, hook copyCleanupHook) (resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.inspect(ctx, id)
	if err != nil {
		return err
	}
	if r.Source != owner || !validCopyID(id) {
		return ErrAuthority
	}
	if r.State == "released" {
		return nil
	}
	var retained int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM worker_change_sets WHERE copy_id=? AND state!='abandoned'`, id).Scan(&retained); err != nil {
		return err
	}
	if retained != 0 {
		return ErrConflict
	}
	defer func() {
		if resultErr != nil {
			saveErr := s.writeCopyCleanupState(context.Background(), id, "uncertain", false)
			resultErr = errors.Join(resultErr, saveErr)
		}
	}()
	ownership, err := s.copyOwnership(ctx, id)
	if err != nil {
		return err
	}
	if ownership.InUse {
		return ErrHostUnavailable
	}
	if ownership.Identity == "" {
		if _, err = s.root.Lstat(id); os.IsNotExist(err) {
			return s.finishCopyRelease(ctx, id)
		}
		return ErrRevision
	}
	ownership, root, err := s.quarantineCopy(ctx, id, ownership, hook)
	if os.IsNotExist(err) && ownership.Empty && ownership.Detached {
		return s.finishDetachedCopyRelease(ctx, id)
	}
	if err != nil {
		return err
	}
	defer root.Close()
	if ownership.Empty {
		if err = verifyEmptyCopyRoot(root); err != nil {
			return err
		}
		if err = syncCopyCleanupDirectory(root); err != nil {
			return err
		}
	} else {
		// Matches measurement's content plus two retained-artifact allowances,
		// with fixed headroom for the enclosing supervisor metadata directories.
		entries, err := inventoryCopyCleanup(ctx, root, 35*s.policy.MaxEntries+128)
		if err != nil {
			return err
		}
		if err = s.writeCopyCleanupState(ctx, id, "releasing", false); err != nil {
			return err
		}
		if err = removeCopyEntries(ctx, root, entries, hook); err != nil {
			return err
		}
		if err = verifyEmptyCopyRoot(root); err != nil {
			return err
		}
		before := ownership
		ownership.Empty = true
		if err = s.writeCopyOwnership(ctx, id, before, ownership); err != nil {
			return err
		}
		if err = cleanupBoundary(hook, "empty_saved", ownership.Quarantine); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	current, err := s.ownedCopyRoot(ownership.Quarantine, ownership.Identity)
	if err != nil {
		return err
	}
	current.Close()
	expected, err := root.Stat(".")
	if err != nil {
		return err
	}
	if err = cleanupBoundary(hook, "before_directory_remove", ownership.Quarantine); err != nil {
		return err
	}
	if err = removeCopyEntry(s.root, ownership.Quarantine, expected); err != nil {
		return err
	}
	if err = root.Close(); err != nil {
		return err
	}
	if err = cleanupBoundary(hook, "directory_removed", ownership.Quarantine); err != nil {
		return err
	}
	if _, err = s.root.Lstat(ownership.Quarantine); !os.IsNotExist(err) {
		return ErrRevision
	}
	return s.finishDetachedCopyRelease(ctx, id)
}

func (s *WorkingCopies) finishDetachedCopyRelease(ctx context.Context, id string) error {
	// An unexpected old-path entry belongs to neither this quarantined copy
	// nor this cleanup attempt. Retain it and the unresolved reservation.
	if _, err := s.root.Lstat(id); !os.IsNotExist(err) {
		return ErrRevision
	}
	if err := syncCopyCleanupDirectory(s.root); err != nil {
		return err
	}
	return s.finishCopyRelease(ctx, id)
}

func syncCopyCleanupDirectory(root *os.Root) error {
	parent, err := pinReplacementDirectory(root, ".")
	if err != nil {
		return err
	}
	defer parent.close()
	// Reuse the existing platform persistence contract: directory fsync on
	// Linux; Windows power-loss durability remains separately unqualified.
	return syncReplacement(parent, parent)
}

func checkedCopyEntry(root *os.Root, name string, expected os.FileInfo) error {
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if current.Mode().Type() != expected.Mode().Type() || !os.SameFile(expected, current) || current.Mode().IsRegular() && (current.Size() != expected.Size() || !current.ModTime().Equal(expected.ModTime())) {
		return ErrRevision
	}
	return nil
}

func (s *WorkingCopies) finishCopyRelease(ctx context.Context, id string) error {
	return s.writeCopyCleanupState(ctx, id, "released", true)
}

type copyCleanupEntry struct {
	name string
	info os.FileInfo
}

func inventoryCopyCleanup(ctx context.Context, root *os.Root, limit int) ([]copyCleanupEntry, error) {
	var entries []copyCleanupEntry
	err := walkWorkingCopy(ctx, root, limit, func(name string, info os.FileInfo) error {
		entries = append(entries, copyCleanupEntry{name, info})
		return nil
	})
	return entries, err
}

func removeCopyEntries(ctx context.Context, root *os.Root, entries []copyCleanupEntry, hook copyCleanupHook) error {
	for i := len(entries) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := entries[i]
		if err := cleanupBoundary(hook, "before_entry_remove", entry.name); err != nil {
			return err
		}
		if err := removeCopyEntry(root, entry.name, entry.info); err != nil {
			return err
		}
		if err := cleanupBoundary(hook, "entry_removed", entry.name); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func verifyEmptyCopyRoot(root *os.Root) error {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer file.Close()
	entries, err := file.Readdirnames(1)
	if len(entries) != 0 {
		return ErrRevision
	}
	if err != io.EOF {
		return errors.Join(ErrRevision, err)
	}
	return nil
}
