package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
)

// The caller must persist dispatch intent before calling replacePreserving and
// retain both private names after any dispatched error. This primitive never
// retries, deletes displaced bytes, rolls back, or claims task completion.
// private is a supervisor-owned directory outside all worker-visible roots.
type preservedReplacement struct {
	Dispatched      bool
	Displaced       string
	DisplacedDigest string
	ResultDigest    string
}

type replacementDirectory struct {
	file  *os.File
	path  string
	close func() error
}

func replacementFileDigest(ctx context.Context, root *os.Root, name string, limit int64) (string, error) {
	digest, _, err := observeReplacementFile(ctx, root, name, limit)
	return digest, err
}

func observeReplacementFile(ctx context.Context, root *os.Root, name string, limit int64) (string, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		return "", nil, ErrUnsupported
	}
	f, err := openWorkingCopyFile(root, name, info)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(copyContextReader{ctx, f}, limit+1))
	if err != nil {
		return "", nil, err
	}
	after, err := f.Stat()
	if err != nil || n != info.Size() || n > limit || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", nil, ErrRevision
	}
	return hex.EncodeToString(h.Sum(nil)), after, nil
}

func replacePreserving(ctx context.Context, target *os.Root, name string, private *os.Root, staged, backup, expected, result string, limit int64) (receipt preservedReplacement, resultErr error) {
	return replacePreservingUsing(ctx, target, name, private, staged, backup, expected, result, limit, replacePinned)
}

type replacementDispatch func(*replacementDirectory, string, *replacementDirectory, string, string) (string, error)

func replacePreservingUsing(ctx context.Context, target *os.Root, name string, private *os.Root, staged, backup, expected, result string, limit int64, dispatch replacementDispatch) (receipt preservedReplacement, resultErr error) {
	if !validCandidatePath(name) || !validCandidatePath(staged) || !validCandidatePath(backup) || path.Dir(staged) != "." || path.Dir(backup) != "." || staged == backup || !validCopyID(expected) || !validCopyID(result) || limit < 1 || limit > DefaultWorkingCopyPolicy().MaxCopyBytes {
		return receipt, ErrProtocol
	}
	if err := PrivateStateOutsideRoot(target.Name(), private.Name()); err != nil {
		return receipt, err
	}
	if _, err := private.Lstat(backup); !os.IsNotExist(err) {
		if err == nil {
			err = ErrConflict
		}
		return receipt, err
	}
	parentName := path.Dir(name)
	targetDir, err := pinReplacementDirectory(target, parentName)
	if err != nil {
		return receipt, err
	}
	defer targetDir.close()
	privateDir, err := pinReplacementDirectory(private, ".")
	if err != nil {
		return receipt, err
	}
	defer privateDir.close()
	if err = qualifyReplacementVolume(targetDir, privateDir); err != nil {
		return receipt, err
	}
	// Use pinned directories for pre/post reads too, so observation cannot drift
	// to another directory if an ancestor is renamed on Unix.
	targetView, err := replacementView(target, path.Dir(name), targetDir)
	if err != nil {
		return receipt, err
	}
	defer targetView.Close()
	privateView, err := replacementView(private, ".", privateDir)
	if err != nil {
		return receipt, err
	}
	defer privateView.Close()
	name = path.Base(name)
	before, err := replacementFileDigest(ctx, targetView, name, limit)
	if err != nil {
		return receipt, err
	}
	if before != expected {
		return receipt, ErrRevision
	}
	proposed, err := replacementFileDigest(ctx, privateView, staged, limit)
	if err != nil {
		return receipt, err
	}
	if proposed != result {
		return receipt, ErrRevision
	}
	if err = ctx.Err(); err != nil {
		return receipt, err
	}
	receipt.Dispatched = true
	receipt.Displaced, err = dispatch(targetDir, name, privateDir, staged, backup)
	if err != nil {
		return receipt, &MutationError{Cause: err, Effects: "unknown"}
	}
	// Cancellation after dispatch cannot erase actual effects. Preserve both
	// versions and surface conflict; never overwrite a newer edit to roll back.
	observe := context.WithoutCancel(ctx)
	var displacedInfo, resultInfo os.FileInfo
	receipt.DisplacedDigest, displacedInfo, err = observeReplacementFile(observe, privateView, receipt.Displaced, limit)
	if err != nil {
		return receipt, &MutationError{Cause: err, Effects: "unknown"}
	}
	receipt.ResultDigest, resultInfo, err = observeReplacementFile(observe, targetView, name, limit)
	if err != nil {
		return receipt, &MutationError{Cause: err, Effects: "unknown"}
	}
	if receipt.DisplacedDigest != expected || receipt.ResultDigest != result {
		return receipt, &MutationError{Cause: ErrConflict, Effects: "partial"}
	}
	// A pinned directory can move on Unix. Its receipts describe that object,
	// but the requested path must still name it before claiming integration.
	currentView, err := replacementView(target, parentName, targetDir)
	if err != nil {
		return receipt, &MutationError{Cause: errors.Join(ErrConflict, err), Effects: "partial"}
	}
	currentView.Close()
	if err = errors.Join(syncReplacementFile(targetView, name, resultInfo), syncReplacementFile(privateView, receipt.Displaced, displacedInfo)); err != nil {
		return receipt, &MutationError{Cause: err, Effects: "unknown"}
	}
	if err = syncReplacement(targetDir, privateDir); err != nil {
		return receipt, &MutationError{Cause: err, Effects: "unknown"}
	}
	return receipt, nil
}

func syncReplacementFile(root *os.Root, name string, expected os.FileInfo) error {
	// Sync exactly the regular object whose post-dispatch bytes were observed.
	// A changed pathname is uncertainty, never a reason to follow or delete it.
	f, err := openWorkingCopyObject(root, name, expected, true)
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	after, statErr := f.Stat()
	if statErr == nil && (after.Size() != expected.Size() || !after.ModTime().Equal(expected.ModTime())) {
		statErr = ErrRevision
	}
	return errors.Join(syncErr, statErr, f.Close())
}

func closeReplacementFiles(files []*os.File) error {
	var err error
	for i := len(files) - 1; i >= 0; i-- {
		err = errors.Join(err, files[i].Close())
	}
	return err
}

func replacementView(root *os.Root, name string, pinned *replacementDirectory) (*os.Root, error) {
	view, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	actual, err := view.Stat(".")
	if err == nil {
		var expected os.FileInfo
		expected, err = pinned.file.Stat()
		if err == nil && !os.SameFile(actual, expected) {
			err = ErrPath
		}
	}
	if err != nil {
		view.Close()
		return nil, err
	}
	return view, nil
}
