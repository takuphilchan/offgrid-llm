package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// WorkspaceID is an opaque UI namespace, not an authentication credential.
// Call only while holding exclusive workspace ownership. Backups retain it.
func (o *Ownership) WorkspaceID() (string, error) {
	path := filepath.Join(filepath.Dir(o.file.Name()), "workspace-id")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() != 32 {
			return "", fmt.Errorf("invalid workspace identity file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if _, err := hex.DecodeString(string(data)); err != nil {
			return "", fmt.Errorf("invalid workspace identity")
		}
		return string(data), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(data[:])
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, err = file.WriteString(id)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	return id, closeErr
}
