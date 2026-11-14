package execution

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// Shared by native CLI and desktop supervisors for this OS account. This is a
// project identity, never an authentication key or a reusable execution grant.
func ProjectIdentityDirectory() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "offgrid", "project-host"), nil
}

func loadProjectHostIdentity(directory, approvedRoot string) (string, error) {
	if !filepath.IsAbs(directory) {
		return "", ErrPath
	}
	if err := PrivateStateOutsideRoot(approvedRoot, directory); err != nil {
		return "", err
	}
	if err := EnsurePrivateDirectory(directory); err != nil {
		return "", err
	}
	owner, err := storage.AcquireOwnership(directory)
	if err != nil {
		return "", err
	}
	defer owner.Close()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	info, err := root.Lstat("identity")
	if errors.Is(err, os.ErrNotExist) {
		id, err := opaqueToken()
		if err != nil {
			return "", err
		}
		f, err := root.OpenFile("identity", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		_, writeErr := f.WriteString(id)
		err = errors.Join(writeErr, f.Sync(), f.Close())
		if err != nil {
			return "", err
		}
		return id, nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() != 64 {
		return "", ErrRevision
	}
	f, err := openWorkingCopyFile(root, "identity", info)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65))
	if err != nil {
		return "", err
	}
	if !validCredential(string(data)) {
		return "", ErrRevision
	}
	return string(data), nil
}
