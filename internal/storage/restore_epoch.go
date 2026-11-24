package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// RestoreEpochFilename is service-owned activation metadata, not a credential.
// Every restore replaces it after verifying the archived bytes. Consumers must
// bind restored grants to a newly reviewed epoch rather than the retained
// workspace-id or a previously enabled snapshot.
const RestoreEpochFilename = ".offgrid-restore-epoch"

func ReadRestoreEpoch(directory string) (string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	before, err := root.Lstat(RestoreEpochFilename)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Size() != 64 {
		return "", errors.New("workspace restore epoch is not a valid regular file")
	}
	file, err := openBackupSourceFile(root, RestoreEpochFilename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != 64 || !os.SameFile(before, opened) {
		return "", errors.New("workspace restore epoch changed during observation")
	}
	raw, err := io.ReadAll(io.LimitReader(file, 65))
	if err != nil {
		return "", err
	}
	value := string(raw)
	if len(raw) != 64 || value != strings.ToLower(value) {
		return "", errors.New("workspace restore epoch is malformed")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", errors.New("workspace restore epoch is malformed")
	}
	return value, nil
}

// Only the freshly owned restore stage is modified. A source archive's epoch
// has already been authenticated and verified as historical data; it cannot
// authorize the new restored workspace, even if its bytes are malformed.
func refreshRestoreEpoch(ctx context.Context, root *os.Root) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("create restore activation epoch: %w", err)
	}
	if before, err := root.Lstat(RestoreEpochFilename); err == nil {
		if !before.Mode().IsRegular() {
			return errors.New("reserved restore epoch path is not a regular file")
		}
		if err := root.Remove(RestoreEpochFilename); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := root.OpenFile(RestoreEpochFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.WriteString(hex.EncodeToString(nonce[:]))
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
