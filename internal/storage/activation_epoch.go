package storage

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
)

// EnsureWorkspaceActivationEpoch creates the initial nonempty activation
// identity during explicit grant review. Read-only status never calls it.
// The caller holds the existing workspace Ownership lease. Existing invalid
// metadata is preserved for diagnosis, never silently replaced.
func EnsureWorkspaceActivationEpoch(directory string) (_ string, resultErr error) {
	current, err := ReadRestoreEpoch(directory)
	if err != nil || current != "" {
		return current, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	value := hex.EncodeToString(nonce[:])
	file, err := createPrivateBackupFile(directory)
	if err != nil {
		return "", err
	}
	identity, err := file.Stat()
	if err != nil {
		file.Close()
		return "", &BackupRetainedError{Path: file.Name(), Cause: err}
	}
	published := false
	defer func() {
		if !published {
			cleanupBackupPath(file.Name(), false, identity, &resultErr)
		}
	}()
	_, err = file.WriteString(value)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err := backupPathIdentity(file.Name(), identity); err != nil {
		return "", err
	}
	err = publishDirectory(file.Name(), filepath.Join(directory, RestoreEpochFilename))
	if err != nil {
		if os.IsExist(err) {
			return ReadRestoreEpoch(directory)
		}
		return "", err
	}
	published = true
	if err := syncBackupParent(directory); err != nil {
		return "", err
	}
	return value, nil
}
