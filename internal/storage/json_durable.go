package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteDurableJSON publishes a private synced snapshot and confirms namespace
// durability. Errors may occur after publication; callers must not assume the
// old value survived or continue using cached authority after an error.
// The caller must hold the workspace's existing exclusive Ownership lease.
func WriteDurableJSON(path string, value any) error {
	return writeDurableJSON(path, value, replaceDurableSnapshot)
}

func writeDurableJSON(path string, value any, publish func(string, string) (bool, error)) (resultErr error) {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := createPrivateBackupFile(filepath.Dir(path))
	if err != nil {
		return err
	}
	identity, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	published := false
	defer func() {
		if !published {
			cleanupBackupPath(file.Name(), false, identity, &resultErr)
		}
	}()
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := backupPathIdentity(file.Name(), identity); err != nil {
		return err
	}
	published, err = publish(file.Name(), path)
	return err
}
