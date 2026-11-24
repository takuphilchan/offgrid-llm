package storage

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	BackupFormat                  = 1
	maxBackupFiles                = 100000
	maxBackupBytes          int64 = 100 << 30
	maxManifestBytes        int64 = 32 << 20
	maxBackupPathBytes            = 4096
	maxBackupPathDepth            = 128
	maxBackupPathEntries          = 2*maxBackupFiles + 2
	maxBackupPathIndexBytes int64 = 64 << 20
)

// Backups contain private data and credentials. A digest detects damage, not
// malicious replacement: accept archives only from a trusted local source.
type BackupManifest struct {
	// Transport metadata is returned to callers, never written into format-1 ZIP.
	Encryption         string       `json:"-"`
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
	return BackupWorkspaceWithOptions(ctx, directory, destination, version, BackupWriteOptions{})
}

func BackupWorkspaceWithOptions(ctx context.Context, directory, destination, version string, options BackupWriteOptions) (*BackupManifest, error) {
	return BackupWorkspaceThen(ctx, directory, destination, version, options, nil)
}

// BackupWorkspaceThen retains exclusive workspace ownership through a follow-up
// operation. The callback runs only after verified, durable archive publication;
// failure returns the retained manifest and never restores/replays automatically.
// The callback must use an already-owned implementation, not acquire a new lock.
func BackupWorkspaceThen(ctx context.Context, directory, destination, version string, options BackupWriteOptions, after func(*BackupManifest) error) (_ *BackupManifest, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := options.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(version) == "" || len(version) > 256 {
		return nil, fmt.Errorf("application version is required and must be at most 256 bytes")
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
	inputRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer inputRoot.Close()
	rootIdentity, err := inputRoot.Stat(".")
	if err != nil {
		return nil, err
	}
	file, err := createPrivateBackupFile(parent)
	if err != nil {
		return nil, err
	}
	fileIdentity, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, &BackupRetainedError{Path: file.Name(), Cause: err}
	}
	cleanupFile := true
	defer func() {
		_ = file.Close()
		if cleanupFile {
			cleanupBackupPath(file.Name(), false, fileIdentity, &resultErr)
		}
	}()
	cipher, err := newBackupCipher(ctx, file, options)
	if err != nil {
		return nil, err
	}
	defer cipher.clearKey()
	if err := backupBoundary(ctx, "backup_opened", file.Name()); err != nil {
		return nil, err
	}
	archive := zip.NewWriter(cipher.writer)
	manifest := &BackupManifest{Format: BackupFormat, ApplicationVersion: version, CreatedAt: time.Now().UTC(), Scope: "workspace-data-directory", Files: []BackupFile{}}
	paths := backupPaths{}
	if err := paths.add("manifest.json"); err != nil {
		return nil, err
	}
	directoryBytes := int64(46 + len("manifest.json") + 28)
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
		if err := paths.add("state/" + relative); err != nil {
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
		directoryBytes += int64(46 + len("state/") + len(relative) + 28)
		if directoryBytes > maxBackupDirectoryBytes {
			return fmt.Errorf("workspace exceeds backup metadata limits")
		}
		if err := backupBoundary(ctx, "backup_source_observed", filename); err != nil {
			return err
		}
		input, err := openBackupSourceFile(inputRoot, filepath.FromSlash(relative))
		if err != nil {
			return err
		}
		defer input.Close()
		opened, err := input.Stat()
		if err != nil {
			return err
		}
		observed, err := inputRoot.Lstat(filepath.FromSlash(relative))
		if err != nil {
			return err
		}
		if !opened.Mode().IsRegular() || !observed.Mode().IsRegular() || !os.SameFile(info, opened) || !os.SameFile(opened, observed) {
			return fmt.Errorf("workspace source identity changed during backup: %s", relative)
		}
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
		observed, err = inputRoot.Lstat(filepath.FromSlash(relative))
		if err != nil || !observed.Mode().IsRegular() || !os.SameFile(opened, after) || !os.SameFile(after, observed) {
			return fmt.Errorf("workspace source identity changed during backup: %s", relative)
		}
		manifest.Files = append(manifest.Files, BackupFile{Path: relative, Size: size, SHA256: hex.EncodeToString(digest.Sum(nil)), Executable: info.Mode().Perm()&0111 != 0})
		manifest.Bytes += size
		return nil
	})
	if err != nil {
		return nil, errors.Join(err, archive.Close(), cipher.close())
	}
	writer, err := archive.Create("manifest.json")
	if err == nil {
		err = json.NewEncoder(&contextBackupWriter{ctx, writer, maxManifestBytes}).Encode(manifest)
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := cipher.close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = backupBoundary(ctx, "backup_before_sync", file.Name())
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
	if err := backupBoundary(ctx, "backup_closed", file.Name()); err != nil {
		return nil, err
	}
	if err := backupPathIdentity(file.Name(), fileIdentity); err != nil {
		return nil, err
	}
	sealedFile, err := os.Lstat(file.Name())
	if err != nil {
		return nil, err
	}
	// Verify the completed archive before platform-specific no-replace publication.
	if _, err := VerifyBackupWithOptions(ctx, file.Name(), cipher.verificationOptions()); err != nil {
		return nil, err
	}
	if err := backupBoundary(ctx, "backup_verified", file.Name()); err != nil {
		return nil, err
	}
	if err := backupPathIdentity(file.Name(), fileIdentity); err != nil {
		return nil, err
	}
	currentFile, err := os.Lstat(file.Name())
	if err != nil {
		return nil, err
	}
	if currentFile.Size() != sealedFile.Size() || !currentFile.ModTime().Equal(sealedFile.ModTime()) {
		return nil, errors.New("verified backup archive changed before publication")
	}
	if err := publishBackupFile(file.Name(), destination); err != nil {
		return nil, fmt.Errorf("publish backup without overwriting: %w", err)
	}
	if backupPublicationMovesSource {
		cleanupFile = false
	}
	manifest.Encryption = cipher.encryption
	if err := backupBoundary(ctx, "backup_published", destination); err != nil {
		return manifest, fmt.Errorf("backup published at %s; completion confirmation interrupted: %w", destination, err)
	}
	if err := syncBackupParent(parent); err != nil {
		return manifest, fmt.Errorf("backup published at %s but durability confirmation failed: %w", destination, err)
	}
	if after != nil {
		if err := ctx.Err(); err != nil {
			return manifest, fmt.Errorf("backup published at %s; follow-up cancelled: %w", destination, err)
		}
		if err := backupPathIdentity(root, rootIdentity); err != nil {
			return manifest, fmt.Errorf("backup published at %s; workspace identity changed before follow-up: %w", destination, err)
		}
		if err := after(manifest); err != nil {
			return manifest, fmt.Errorf("backup published at %s; follow-up operation failed: %w", destination, err)
		}
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
	if len(name) > maxBackupPathBytes || strings.Count(name, "/") >= maxBackupPathDepth || !utf8.ValidString(name) {
		return fmt.Errorf("backup path exceeds byte/depth limits or is not UTF-8")
	}
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
type backupPathNode struct {
	name     string
	file     bool
	children map[string]*backupPathNode
}

type backupPaths struct {
	root    backupPathNode
	entries int
	bytes   int64
}

func (p *backupPaths) add(name string) error {
	if err := portablePath(name); err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	node := &p.root
	for i, part := range parts {
		// Normalize bounded components, never repeatedly allocate full path
		// prefixes. Entries and retained strings have separate global budgets.
		if int64(len(part))*8 > maxBackupPathIndexBytes-p.bytes {
			return fmt.Errorf("backup path index exceeds byte limit")
		}
		key := strings.ToLower(norm.NFC.String(part))
		last := i == len(parts)-1
		if previous, ok := node.children[key]; ok {
			if previous.name != part || previous.file || last {
				return fmt.Errorf("colliding backup path: %s", name)
			}
			node = previous
			continue
		}
		if p.entries >= maxBackupPathEntries {
			return fmt.Errorf("backup path index exceeds entry limit")
		}
		if node.children == nil {
			node.children = make(map[string]*backupPathNode)
		}
		// Clone components so a short leaf does not retain an entire long path
		// through a substring, bypassing the retained-string accounting.
		child := &backupPathNode{name: strings.Clone(part), file: last}
		node.children[strings.Clone(key)], node = child, child
		p.entries++
		p.bytes += int64(len(part) + len(key))
	}
	return nil
}

func readBackupManifest(archive *zip.Reader) (*BackupManifest, map[string]*zip.File, error) {
	if len(archive.File) > maxBackupFiles+1 {
		return nil, nil, fmt.Errorf("backup contains too many files")
	}
	files := make(map[string]*zip.File)
	seen := make(map[string]bool)
	paths := backupPaths{}
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
	manifest, err := decodeBackupManifest(reader, len(files))
	if err != nil {
		return nil, nil, err
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
	return manifest, files, nil
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
	return VerifyBackupWithOptions(ctx, filename, BackupReadOptions{})
}

func VerifyBackupWithOptions(ctx context.Context, filename string, options BackupReadOptions) (*BackupManifest, error) {
	if err := ctx.Err(); err != nil {
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
	for _, item := range manifest.Files {
		if err := copyBackupFile(ctx, files[item.Path], item, io.Discard); err != nil {
			return nil, err
		}
	}
	manifest.Encryption = archive.encryption
	return manifest, nil
}
