package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// SQLite validation may checkpoint WAL contents after extraction's first sync.
// Flush the final staged bytes, then directory entries from children to parents.
func syncRestoredBackup(ctx context.Context, root string) error {
	var directories []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("restored staging contains an unsupported file")
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return errors.Join(file.Sync(), file.Close())
	}); err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncBackupParent(directories[i]); err != nil {
			return err
		}
	}
	return nil
}
