package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RestoreWorkspace never overwrites a workspace. It validates a complete staged
// copy and atomically installs it at a new path. Keeping the original untouched
// makes recovery reversible without deleting newer work or mixing schema versions.
func RestoreWorkspace(ctx context.Context, filename, target, version string) (*BackupManifest, error) {
	return RestoreWorkspaceWithOptions(ctx, filename, target, version, BackupReadOptions{})
}

func RestoreWorkspaceWithOptions(ctx context.Context, filename, target, version string, options BackupReadOptions) (_ *BackupManifest, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent, err := existingDirectory(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	target = filepath.Join(parent, filepath.Base(target))
	if _, err := os.Lstat(target); err == nil {
		return nil, fmt.Errorf("restore requires a new, nonexistent data directory; the current workspace is never overwritten")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	archive, err := openBackupArchive(ctx, filename, options)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	manifest, files, err := readBackupManifest(archive.Reader)
	if err != nil {
		return nil, err
	}
	if version == "dev" || version == "" || version != manifest.ApplicationVersion {
		return nil, fmt.Errorf("restore requires the matching application version %s; current version is %s", manifest.ApplicationVersion, version)
	}
	if err := backupBoundary(ctx, "restore_authenticated", filename); err != nil {
		return nil, err
	}
	stage, err := createPrivateRestoreDirectory(parent)
	if err != nil {
		return nil, err
	}
	// Only this freshly created, private directory is recursively removed.
	stageIdentity, err := os.Lstat(stage)
	if err != nil {
		return nil, &BackupRetainedError{Path: stage, Directory: true, Cause: err}
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			cleanupBackupPath(stage, true, stageIdentity, &resultErr)
		}
	}()
	stageRoot, err := os.OpenRoot(stage)
	if err != nil {
		return nil, err
	}
	defer stageRoot.Close()
	if err := backupBoundary(ctx, "restore_staged", stage); err != nil {
		return nil, err
	}
	for _, item := range manifest.Files {
		if err := backupPathIdentity(stage, stageIdentity); err != nil {
			return nil, err
		}
		destination := filepath.Join(stage, filepath.FromSlash(item.Path))
		if !within(stage, destination) {
			return nil, fmt.Errorf("restore path escapes staging")
		}
		if err := stageRoot.MkdirAll(filepath.Dir(filepath.FromSlash(item.Path)), 0700); err != nil {
			return nil, err
		}
		file, err := stageRoot.OpenFile(filepath.FromSlash(item.Path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		err = copyBackupFile(ctx, files[item.Path], item, file)
		if err == nil && item.Executable {
			err = file.Chmod(0700)
		}
		if err == nil {
			err = file.Sync()
		}
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		if err := backupBoundary(ctx, "restore_file_synced", destination); err != nil {
			return nil, err
		}
	}
	// A valid ZIP hash is not proof of a usable SQLite database. WAL recovery
	// and integrity checks happen on staging, never on the original workspace.
	var databases []string
	for _, item := range manifest.Files {
		if strings.HasSuffix(item.Path, ".db") || strings.HasSuffix(item.Path, ".sqlite") {
			databases = append(databases, item.Path)
		}
	}
	sort.Strings(databases)
	for _, name := range databases {
		if err := backupPathIdentity(stage, stageIdentity); err != nil {
			return nil, err
		}
		db, err := OpenSQLite(filepath.Join(stage, filepath.FromSlash(name)))
		if err != nil {
			return nil, fmt.Errorf("verify restored database %s: %w", name, err)
		}
		err = CheckSQLite(ctx, db)
		if closeErr := db.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, fmt.Errorf("verify restored database %s: %w", name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Historical archive bytes have been verified. A restored workspace gets
	// fresh activation metadata so archived grants cannot silently roll back
	// later revocations. The returned manifest still describes the archive.
	if err := refreshRestoreEpoch(ctx, stageRoot); err != nil {
		return nil, err
	}
	if err := backupBoundary(ctx, "restore_epoch_written", stage); err != nil {
		return nil, err
	}
	if err := syncRestoredBackup(ctx, stage); err != nil {
		return nil, err
	}
	// Windows directory-root handles intentionally deny rename while extraction
	// is active. Release the completed traversal handle before no-replace move.
	if err := stageRoot.Close(); err != nil {
		return nil, err
	}
	if err := backupBoundary(ctx, "restore_verified", stage); err != nil {
		return nil, err
	}
	if err := backupPathIdentity(stage, stageIdentity); err != nil {
		return nil, err
	}
	// Reserve the destination while activation occurs. Never rename over a
	// path another process created while the archive was being checked.
	if err := publishDirectory(stage, target); err != nil {
		return nil, err
	}
	cleanupStage = false
	manifest.Encryption = archive.encryption
	if err := backupBoundary(ctx, "restore_published", target); err != nil {
		return manifest, fmt.Errorf("workspace restored at %s; completion confirmation interrupted: %w", target, err)
	}
	if err := syncBackupParent(parent); err != nil {
		return manifest, fmt.Errorf("workspace restored at %s but durability confirmation failed: %w", target, err)
	}
	return manifest, nil
}
