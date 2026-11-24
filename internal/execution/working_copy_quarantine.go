package execution

import (
	"context"
	"os"
	"strings"
)

type copyCleanupHook func(string, string) error

func cleanupBoundary(hook copyCleanupHook, stage, name string) error {
	if hook != nil {
		return hook(stage, name)
	}
	return nil
}

func validCleanupName(name string) bool {
	return strings.HasPrefix(name, "cleanup-") && validCopyID(strings.TrimPrefix(name, "cleanup-"))
}

// Durable intent precedes rename. Only an exact post-rename identity permits
// deletion; a raced replacement is retained at quarantine, never rolled back.
func (s *WorkingCopies) quarantineCopy(ctx context.Context, id string, ownership workingCopyOwnership, hook copyCleanupHook) (workingCopyOwnership, *os.Root, error) {
	if ownership.Quarantine == "" {
		source, err := s.ownedCopyRoot(id, ownership.Identity)
		if err != nil {
			return ownership, nil, err
		}
		source.Close()
		token, err := opaqueToken()
		if err != nil {
			return ownership, nil, err
		}
		before := ownership
		ownership.Quarantine = "cleanup-" + token
		if err = s.writeCopyOwnership(ctx, id, before, ownership); err != nil {
			return ownership, nil, err
		}
		if err = cleanupBoundary(hook, "intent_saved", id); err != nil {
			return ownership, nil, err
		}
	}
	root, err := s.ownedCopyRoot(ownership.Quarantine, ownership.Identity)
	if os.IsNotExist(err) && !ownership.Detached {
		source, sourceErr := s.ownedCopyRoot(id, ownership.Identity)
		if sourceErr != nil {
			return ownership, nil, sourceErr
		}
		source.Close()
		parent, pinErr := pinReplacementDirectory(s.root, ".")
		if pinErr != nil {
			return ownership, nil, pinErr
		}
		defer parent.close()
		if err = ctx.Err(); err != nil {
			return ownership, nil, err
		}
		if err = cleanupBoundary(hook, "before_detach", id); err != nil {
			return ownership, nil, err
		}
		if err = movePinnedExclusive(parent, id, parent, ownership.Quarantine); err != nil {
			return ownership, nil, err
		}
		if err = cleanupBoundary(hook, "after_detach", ownership.Quarantine); err != nil {
			return ownership, nil, err
		}
		root, err = s.ownedCopyRoot(ownership.Quarantine, ownership.Identity)
	}
	if err != nil {
		return ownership, nil, err
	}
	if !ownership.Detached {
		if err = syncCopyCleanupDirectory(s.root); err != nil {
			root.Close()
			return ownership, nil, err
		}
		before := ownership
		ownership.Detached = true
		if err = s.writeCopyOwnership(ctx, id, before, ownership); err != nil {
			root.Close()
			return ownership, nil, err
		}
	}
	if err = cleanupBoundary(hook, "detachment_saved", ownership.Quarantine); err != nil {
		root.Close()
		return ownership, nil, err
	}
	return ownership, root, nil
}
