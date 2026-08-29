package server

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const dataLayoutVersion = 1

// prepareDataLayout creates the single application-state root and copies
// legacy state into it without overwriting newer files. Legacy files are kept
// in place so an interrupted migration is recoverable.
func prepareDataLayout(modelsDir, dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	legacyRoot := filepath.Clean(filepath.Join(modelsDir, ".."))
	homeDir, _ := os.UserHomeDir()
	migrations := []struct {
		from string
		to   string
	}{
		{from: filepath.Join(legacyRoot, "data"), to: dataDir},
		{from: filepath.Join(legacyRoot, "sessions"), to: filepath.Join(dataDir, "sessions")},
		{from: filepath.Join(modelsDir, "rag"), to: filepath.Join(dataDir, "rag")},
		{from: filepath.Join(legacyRoot, "audio"), to: filepath.Join(dataDir, "audio")},
	}
	defaultUserData := filepath.Join(homeDir, ".offgrid-llm", "data")
	if samePath(dataDir, defaultUserData) {
		migrations = append(migrations, struct {
			from string
			to   string
		}{from: filepath.Join(homeDir, ".offgrid", "sessions"), to: filepath.Join(dataDir, "sessions")})
	}
	for _, migration := range migrations {
		if samePath(migration.from, migration.to) {
			continue
		}
		if err := copyMissingTree(migration.from, migration.to); err != nil {
			return fmt.Errorf("migrate %s to %s: %w", migration.from, migration.to, err)
		}
	}

	marker := filepath.Join(dataDir, ".layout-version")
	if err := writeFileAtomic(marker, []byte(fmt.Sprintf("%d\n", dataLayoutVersion)), 0o600); err != nil {
		return fmt.Errorf("write data layout marker: %w", err)
	}
	return nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func copyMissingTree(source, destination string) error {
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to migrate symlink %s", source)
	}
	if !info.IsDir() {
		if _, err := os.Lstat(destination); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		return copyFileAtomic(source, destination, info.Mode().Perm())
	}

	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := destination
		if relative != "." {
			target = filepath.Join(destination, relative)
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to migrate symlink %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if _, err := os.Lstat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		return copyFileAtomic(path, target, entryInfo.Mode().Perm())
	})
}

func copyFileAtomic(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".migrate-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if mode == 0 {
		mode = 0o600
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, input); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, destination)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
