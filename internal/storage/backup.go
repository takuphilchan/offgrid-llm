package storage

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	BackupFormat           = 1
	maxBackupFiles         = 100000
	maxBackupBytes   int64 = 100 << 30
	maxManifestBytes int64 = 32 << 20
)

// Backups contain private data and credentials. A digest detects damage, not
// malicious replacement: accept archives only from a trusted local source.
type BackupManifest struct {
	Format             int          `json:"format"`
	ApplicationVersion string       `json:"application_version"`
	CreatedAt          time.Time    `json:"created_at"`
	Scope              string       `json:"scope"`
	Files              []BackupFile `json:"files"`
	Bytes              int64        `json:"bytes"`
}

type BackupFile struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Executable bool   `json:"executable,omitempty"`
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// BackupWorkspace requires exclusive ownership and never copies a live database
// in isolation: the entire stopped workspace (including WALs and blobs) is saved.
// Models and configuration outside directory are not included. The caller must
// retain those separately and explain this boundary to the operator.
func BackupWorkspace(ctx context.Context, directory, destination, version string) (*BackupManifest, error) {
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("application version is required")
	}
	root, err := existingDirectory(directory)
	if err != nil {
		return nil, err
	}
	parent, err := existingDirectory(filepath.Dir(destination))
	if err != nil {
		return nil, err
	}
	if within(root, parent) {
		return nil, fmt.Errorf("backup output must be outside the workspace")
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	if _, err := os.Lstat(destination); err == nil {
		return nil, fmt.Errorf("backup output already exists")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	owner, err := AcquireOwnership(root)
	if err != nil {
		return nil, err
	}
	defer owner.Close()
	file, err := os.CreateTemp(parent, ".offgrid-backup-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	archive := zip.NewWriter(file)
	manifest := &BackupManifest{Format: BackupFormat, ApplicationVersion: version, CreatedAt: time.Now().UTC(), Scope: "workspace-data-directory", Files: []BackupFile{}}
	paths := make(backupPaths)
	err = filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if filename == root {
			return nil
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".offgrid-owner.lock" {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup refuses symbolic link: %s", relative)
		}
		if err := portablePath(relative); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if err := paths.add(relative); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup requires regular files: %s", relative)
		}
		if len(manifest.Files) >= maxBackupFiles || info.Size() > maxBackupBytes-manifest.Bytes {
			return fmt.Errorf("workspace exceeds backup limits")
		}
		input, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer input.Close()
		header := &zip.FileHeader{Name: "state/" + relative, Method: zip.Deflate}
		header.SetMode(0600)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		digest := sha256.New()
		size, err := io.Copy(io.MultiWriter(writer, digest), io.LimitReader(contextReader{ctx, input}, info.Size()+1))
		if err != nil {
			return err
		}
		after, err := input.Stat()
		if err != nil {
			return err
		}
		if size != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return fmt.Errorf("workspace changed during backup: %s", relative)
		}
		manifest.Files = append(manifest.Files, BackupFile{Path: relative, Size: size, SHA256: hex.EncodeToString(digest.Sum(nil)), Executable: info.Mode().Perm()&0111 != 0})
		manifest.Bytes += size
		return nil
	})
	if err != nil {
		archive.Close()
		return nil, err
	}
	writer, err := archive.Create("manifest.json")
	if err == nil {
		err = json.NewEncoder(writer).Encode(manifest)
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
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
	// Verify the completed archive before publishing it. Hard-link publication
	// is atomic and fails if another process has claimed the destination.
	if _, err := VerifyBackup(ctx, file.Name()); err != nil {
		return nil, err
	}
	if err := os.Link(file.Name(), destination); err != nil {
		return nil, fmt.Errorf("publish backup without overwriting: %w", err)
	}
	return manifest, nil
}

func existingDirectory(directory string) (string, error) {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", directory)
	}
	return abs, nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// portablePath avoids traversal, NTFS alternate streams, reserved Windows names,
// and ambiguities when restoring a Linux archive on Windows or macOS.
func portablePath(name string) error {
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.ContainsAny(name, `\:*?"<>|`) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("unsafe backup path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return fmt.Errorf("unsafe backup path %q", name)
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		runes := []rune(base)
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(runes) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsRune("123456789¹²³", runes[3])) {
			return fmt.Errorf("nonportable backup path %q", name)
		}
	}
	return nil
}

// Reject file/directory aliases and case/normalization collisions before any
// extraction, including collisions in parent directories on Windows and macOS.
type backupPaths map[string]struct {
	name string
	file bool
}

func (p backupPaths) add(name string) error {
	parts := strings.Split(name, "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		key := strings.ToLower(norm.NFC.String(prefix))
		last := i == len(parts)-1
		if previous, ok := p[key]; ok {
			if previous.name != prefix || previous.file || last {
				return fmt.Errorf("colliding backup path: %s", name)
			}
			continue
		}
		p[key] = struct {
			name string
			file bool
		}{prefix, last}
	}
	return nil
}

func readBackupManifest(archive *zip.ReadCloser) (*BackupManifest, map[string]*zip.File, error) {
	if len(archive.File) > maxBackupFiles+1 {
		return nil, nil, fmt.Errorf("backup contains too many files")
	}
	files := make(map[string]*zip.File)
	seen := make(map[string]bool)
	paths := make(backupPaths)
	var manifestFile *zip.File
	var total uint64
	for _, file := range archive.File {
		if err := portablePath(file.Name); err != nil {
			return nil, nil, err
		}
		if err := paths.add(file.Name); err != nil {
			return nil, nil, err
		}
		key := strings.ToLower(file.Name)
		if seen[key] || !file.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("duplicate or non-regular backup entry: %s", file.Name)
		}
		seen[key] = true
		if file.UncompressedSize64 > uint64(maxBackupBytes)+uint64(maxManifestBytes)-total {
			return nil, nil, fmt.Errorf("backup exceeds size limit")
		}
		total += file.UncompressedSize64
		if file.Name == "manifest.json" {
			manifestFile = file
			continue
		}
		if !strings.HasPrefix(file.Name, "state/") {
			return nil, nil, fmt.Errorf("unrecognized backup entry: %s", file.Name)
		}
		files[strings.TrimPrefix(file.Name, "state/")] = file
	}
	if manifestFile == nil || manifestFile.UncompressedSize64 > uint64(maxManifestBytes) {
		return nil, nil, fmt.Errorf("missing or oversized backup manifest")
	}
	reader, err := manifestFile.Open()
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	decoder := json.NewDecoder(io.LimitReader(reader, maxManifestBytes+1))
	decoder.DisallowUnknownFields()
	var manifest BackupManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, nil, fmt.Errorf("invalid backup manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, nil, fmt.Errorf("trailing backup manifest data")
	}
	if manifest.Format != BackupFormat || manifest.ApplicationVersion == "" || manifest.CreatedAt.IsZero() || manifest.Scope != "workspace-data-directory" || len(manifest.Files) != len(files) || manifest.Bytes < 0 || manifest.Bytes > maxBackupBytes {
		return nil, nil, fmt.Errorf("unsupported or incomplete backup manifest")
	}
	seen = make(map[string]bool)
	var count int64
	for _, item := range manifest.Files {
		if err := portablePath(item.Path); err != nil {
			return nil, nil, err
		}
		file := files[item.Path]
		digest, err := hex.DecodeString(item.SHA256)
		if file == nil || seen[strings.ToLower(item.Path)] || item.Path == ".offgrid-owner.lock" || item.Size < 0 || item.Size > maxBackupBytes-count || file.UncompressedSize64 != uint64(item.Size) || err != nil || len(digest) != sha256.Size {
			return nil, nil, fmt.Errorf("invalid backup inventory: %s", item.Path)
		}
		seen[strings.ToLower(item.Path)] = true
		count += item.Size
	}
	if count != manifest.Bytes {
		return nil, nil, fmt.Errorf("backup byte count mismatch")
	}
	return &manifest, files, nil
}

func copyBackupFile(ctx context.Context, file *zip.File, item BackupFile, target io.Writer) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(target, digest), io.LimitReader(contextReader{ctx, reader}, item.Size+1))
	if err != nil {
		return err
	}
	if size != item.Size || !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), item.SHA256) {
		return fmt.Errorf("backup checksum mismatch: %s", item.Path)
	}
	return nil
}

func VerifyBackup(ctx context.Context, filename string) (*BackupManifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	manifest, files, err := readBackupManifest(archive)
	if err != nil {
		return nil, err
	}
	for _, item := range manifest.Files {
		if err := copyBackupFile(ctx, files[item.Path], item, io.Discard); err != nil {
			return nil, err
		}
	}
	return manifest, nil
}

// RestoreWorkspace never overwrites a workspace. It validates a complete staged
// copy and atomically installs it at a new path. Keeping the original untouched
// makes recovery reversible without deleting newer work or mixing schema versions.
func RestoreWorkspace(ctx context.Context, filename, target, version string) (*BackupManifest, error) {
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
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	manifest, files, err := readBackupManifest(archive)
	if err != nil {
		return nil, err
	}
	if version == "dev" || version == "" || version != manifest.ApplicationVersion {
		return nil, fmt.Errorf("restore requires the matching application version %s; current version is %s", manifest.ApplicationVersion, version)
	}
	stage, err := os.MkdirTemp(parent, ".offgrid-restore-*")
	if err != nil {
		return nil, err
	}
	// Only this freshly created, private directory is recursively removed.
	defer os.RemoveAll(stage)
	for _, item := range manifest.Files {
		destination := filepath.Join(stage, filepath.FromSlash(item.Path))
		if !within(stage, destination) {
			return nil, fmt.Errorf("restore path escapes staging")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
	// Reserve the destination while activation occurs. Never rename over a
	// path another process created while the archive was being checked.
	if err := publishDirectory(stage, target); err != nil {
		return nil, err
	}
	return manifest, nil
}
