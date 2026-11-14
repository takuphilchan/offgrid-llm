package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Version-one retention policy is conservative: unsettled operations keep their
// entire reservation. Nothing here automatically deletes recovery evidence.
const maxRetainedReplacements = 128
const maxReplacementStoreBytes = 2 << 30

func (j *Journal) replaceFile(ctx context.Context, session *Session, binding Binding, operation, name string, data []byte, expected string, mode os.FileMode, dispatch replacementDispatch) (result WriteResult, resultErr error) {
	if err := PrivateStateOutsideRoot(session.root.Name(), j.directory); err != nil {
		return result, err
	}
	id := digest([]byte("execution-replacement-v1\x00" + operation))
	proposed := digest(data)
	baseline, err := session.root.Lstat(name)
	if err != nil {
		return result, err
	}
	if !baseline.Mode().IsRegular() || baseline.Size() < 0 || baseline.Size() > MaxFileBytes {
		return result, ErrUnsupported
	}
	reserved := 2*int64(len(data)) + baseline.Size()
	// The operation claim must already be durable. Reserve storage and keep a
	// deterministic private directory before creating any artifact or dispatching.
	insert, err := j.db.ExecContext(ctx, `INSERT INTO execution_replacements(operation,directory,target,expected,proposed,reserved_bytes,state)
SELECT ?,?,?,?,?,?,'prepared' WHERE EXISTS(SELECT 1 FROM execution_operations WHERE id=? AND state='claimed')
AND (SELECT count(*) FROM execution_replacements)<?
AND (SELECT coalesce(sum(reserved_bytes),0) FROM execution_replacements)<=?`, operation, id, name, expected, proposed, reserved, operation, maxRetainedReplacements, int64(maxReplacementStoreBytes)-reserved)
	if err != nil {
		return result, err
	}
	if n, err := insert.RowsAffected(); err != nil || n != 1 {
		return result, errors.Join(ErrLimit, err)
	}
	result = WriteResult{Location: session.location(name), ReplacementID: id, Revision: proposed, Bytes: len(data)}
	var receipt preservedReplacement
	defer func() {
		state := "complete"
		if resultErr != nil {
			state = "rejected"
			if receipt.Dispatched {
				state = "uncertain"
			}
			if errors.Is(resultErr, ErrConflict) || errors.Is(resultErr, ErrRevision) {
				state = "conflict"
			}
		}
		encoded, _ := json.Marshal(receipt)
		if err := j.recordReplacement(context.WithoutCancel(ctx), operation, state, encoded, reserved); err != nil {
			if receipt.Dispatched {
				session.Revoke()
				result.Verified = false
				resultErr = &MutationError{Cause: errors.Join(resultErr, err), Effects: "unknown"}
			} else {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()
	if j.reserveReplacement != nil {
		if err = j.reserveReplacement(ctx, reserved); err != nil {
			return result, err
		}
	}
	private, err := j.replacementDirectory(id)
	if err != nil {
		return result, err
	}
	defer private.Close()
	// Keep an immutable proposed version separate from the exchange operand:
	// Linux exchanges that operand with the actual displaced file.
	for _, file := range []string{"proposed", "staged"} {
		f, err := private.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return result, err
		}
		if err = writeAndSync(f, data); err != nil {
			return result, err
		}
	}
	if err = session.authorize(ctx, binding); err != nil {
		return result, err
	}
	// A crash after this commit is an unresolved dispatch. The operation claim
	// prevents replay even if replacement or final receipt acknowledgement is lost.
	if err = j.recordReplacement(ctx, operation, "dispatching", nil, reserved); err != nil {
		return result, err
	}
	receipt, err = replacePreservingUsing(ctx, session.root, filepath.ToSlash(name), private, "staged", "displaced", expected, proposed, MaxFileBytes, dispatch)
	// A competing writer may have enlarged the displaced file. Retain and
	// charge observed growth; a failed inventory cannot reopen storage capacity.
	measured, measureErr := measureCopyTree(context.WithoutCancel(ctx), private, 8)
	if measureErr != nil {
		reserved = maxReplacementStoreBytes
		if receipt.Dispatched {
			err = &MutationError{Cause: errors.Join(err, measureErr), Effects: "unknown"}
		} else {
			err = errors.Join(err, measureErr)
		}
	} else {
		reserved = max(reserved, measured)
	}
	result.OriginalRetained = receipt.DisplacedDigest != ""
	result.Verified = err == nil
	return result, err
}

func (j *Journal) recordReplacement(ctx context.Context, operation, state string, receipt []byte, reserved int64) error {
	result, err := j.db.ExecContext(ctx, `UPDATE execution_replacements SET state=?,receipt=?,reserved_bytes=max(reserved_bytes,?) WHERE operation=?`, state, receipt, reserved, operation)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) replacementDirectory(id string) (*os.Root, error) {
	if !validCopyID(id) {
		return nil, ErrProtocol
	}
	root, err := os.OpenRoot(j.directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err = root.Mkdir("replacements", 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := root.Lstat("replacements")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrPath
	}
	name := filepath.Join("replacements", id)
	if err = root.Mkdir(name, 0700); err != nil {
		return nil, err
	}
	return root.OpenRoot(name)
}
