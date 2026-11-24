package execution

import (
	"context"
	"database/sql"
	"errors"
	"os"
)

// Ownership is host-private metadata, never a worker grant or public receipt.
// Legacy rows are deliberately not adopted from whatever path exists today.
const workingCopyOwnershipSchema = ` CREATE TABLE IF NOT EXISTS worker_copy_ownership(copy_id TEXT PRIMARY KEY REFERENCES worker_copies(id),version INTEGER NOT NULL,identity TEXT NOT NULL,empty INTEGER NOT NULL CHECK(empty IN (0,1)),in_use INTEGER NOT NULL DEFAULT 1 CHECK(in_use IN (0,1)),quarantine TEXT NOT NULL DEFAULT '',detached INTEGER NOT NULL DEFAULT 0 CHECK(detached IN (0,1))); `

const workingCopyOwnershipV12 = ` ALTER TABLE worker_copy_ownership ADD COLUMN in_use INTEGER NOT NULL DEFAULT 1 CHECK(in_use IN (0,1)); ALTER TABLE worker_copy_ownership ADD COLUMN quarantine TEXT NOT NULL DEFAULT ''; ALTER TABLE worker_copy_ownership ADD COLUMN detached INTEGER NOT NULL DEFAULT 0 CHECK(detached IN (0,1)); `

type workingCopyOwnership struct {
	Version    int
	Identity   string
	Empty      bool
	InUse      bool
	Quarantine string
	Detached   bool
}

func (s *WorkingCopies) copyOwnership(ctx context.Context, id string) (workingCopyOwnership, error) {
	return readCopyOwnership(ctx, s.db, id)
}

type copyRowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readCopyOwnership(ctx context.Context, reader copyRowReader, id string) (workingCopyOwnership, error) {
	var ownership workingCopyOwnership
	err := reader.QueryRowContext(ctx, `SELECT version,identity,empty,in_use,quarantine,detached FROM worker_copy_ownership WHERE copy_id=?`, id).Scan(&ownership.Version, &ownership.Identity, &ownership.Empty, &ownership.InUse, &ownership.Quarantine, &ownership.Detached)
	if errors.Is(err, sql.ErrNoRows) || err == nil && ownership.Version != 1 {
		return ownership, ErrUnsupported
	}
	if err == nil && ownership.Empty && ownership.Identity == "" {
		return ownership, ErrRevision
	}
	if err == nil && (ownership.Quarantine != "" && !validCleanupName(ownership.Quarantine) || ownership.Detached && ownership.Quarantine == "") {
		return ownership, ErrRevision
	}
	return ownership, err
}

func (s *WorkingCopies) beginWorkerUse(ctx context.Context, id string, owner Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.inspect(ctx, id)
	if err != nil {
		return err
	}
	if record.Source != owner || record.State != "ready" {
		return ErrAuthority
	}
	before, err := s.copyOwnership(ctx, id)
	if err != nil {
		return err
	}
	if before.Identity == "" || before.InUse || before.Quarantine != "" || before.Empty {
		return ErrRevision
	}
	after := before
	after.InUse = true
	return s.writeCopyOwnership(ctx, id, before, after)
}

func (s *WorkingCopies) settleWorkerUse(ctx context.Context, id string, owner Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.inspect(ctx, id)
	if err != nil {
		return err
	}
	if record.Source != owner {
		return ErrAuthority
	}
	before, err := s.copyOwnership(ctx, id)
	if err != nil {
		return err
	}
	if before.Quarantine != "" {
		return ErrRevision
	}
	after := before
	after.InUse = false
	return s.writeCopyOwnership(ctx, id, before, after)
}

// A failed close is not a release attempt: preserve the exact ownership and
// reservation while making its unresolved state visible to inspection/restart.
func (s *WorkingCopies) recordWorkerUncertain(ctx context.Context, id string, owner Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.inspect(ctx, id)
	if err != nil {
		return err
	}
	if record.Source != owner {
		return ErrAuthority
	}
	return s.writeCopyCleanupState(ctx, id, "uncertain", false)
}

func (s *WorkingCopies) recordCopyOwnership(ctx context.Context, id string) error {
	info, err := s.root.Lstat(id)
	if err != nil {
		return err
	}
	file, err := openWorkingCopyObject(s.root, id, info, false)
	if err != nil {
		return err
	}
	defer file.Close()
	identity, err := folderHandleIdentity(file)
	if err != nil {
		return err
	}
	before, err := s.copyOwnership(ctx, id)
	if err != nil {
		return err
	}
	if before.Identity != "" || before.InUse || before.Quarantine != "" || before.Empty {
		return ErrRevision
	}
	after := before
	after.Identity = identity
	return s.writeCopyOwnership(ctx, id, before, after)
}

// The returned Root pins the exact directory. Deleting its contents therefore
// cannot drift to a different directory recreated at the outer copy pathname.
func (s *WorkingCopies) ownedCopyRoot(id, identity string) (*os.Root, error) {
	info, err := s.root.Lstat(id)
	if err != nil {
		return nil, err
	}
	file, err := openWorkingCopyObject(s.root, id, info, false)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	actualIdentity, err := folderHandleIdentity(file)
	if err != nil {
		return nil, err
	}
	if identity == "" || actualIdentity != identity {
		return nil, ErrRevision
	}
	root, err := s.root.OpenRoot(id)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, ErrRevision
	}
	current, err := s.root.Lstat(id)
	if err != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		root.Close()
		return nil, ErrRevision
	}
	return root, nil
}
