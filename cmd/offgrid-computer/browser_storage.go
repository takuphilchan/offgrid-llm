package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

var errBrowserStorage = errors.New("computer_storage_unavailable")
var browserStorageResourceID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// This fixed one-shot operation belongs to the trusted pack supervisor, not
// the native input protocol. EOF seals the request before filesystem changes.
// It never opens the dispatch journal or adopts/deletes retained resources.
// Cross-client reservations use the existing account's private admission store.
func serveBrowserStorage(ctx context.Context, input io.ReadCloser, output io.Writer, directory string) error {
	stop := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer stop()
	roots, err := readBrowserStorageRequest(input)
	if err != nil || ctx.Err() != nil {
		return errBrowserStorage
	}
	if validateBrowserStorageLocation(ctx, directory, roots) != nil {
		return errBrowserStorage
	}
	registry, err := execution.AccountHostProtectionRegistry()
	if err != nil {
		return errBrowserStorage
	}
	reservation, err := registry.RegisterBrowserRoot(ctx, directory)
	if err != nil {
		return errBrowserStorage
	}
	if prepareBrowserStorage(ctx, directory, roots) != nil || ctx.Err() != nil {
		return errBrowserStorage
	}
	if reservation.Finalize(ctx) != nil || ctx.Err() != nil {
		return errBrowserStorage
	}
	return computer.WriteControlFrame(output, map[string]any{"protocol": 1, "id": "browser-storage", "result": map[string]any{"ready": true, "browser_storage": 1}})
}

func readBrowserStorageRequest(input io.Reader) ([]string, error) {
	var length uint32
	if err := binary.Read(input, binary.BigEndian, &length); err != nil || length == 0 || length > 16384 {
		return nil, errBrowserStorage
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(input, body); err != nil {
		return nil, errBrowserStorage
	}
	if !utf8.Valid(body) {
		return nil, errBrowserStorage
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errBrowserStorage
	}
	seen := map[string]bool{}
	var roots []string
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return nil, errBrowserStorage
		}
		seen[name] = true
		switch name {
		case "protocol":
			var version int
			if decoder.Decode(&version) != nil || version != 1 {
				return nil, errBrowserStorage
			}
		case "kind":
			var kind string
			if decoder.Decode(&kind) != nil || kind != "prepare_browser_storage" {
				return nil, errBrowserStorage
			}
		case "workspace_roots":
			if decoder.Decode(&roots) != nil || roots == nil || len(roots) > 16 {
				return nil, errBrowserStorage
			}
			for _, root := range roots {
				if len(root) > 4096 || !filepath.IsAbs(root) {
					return nil, errBrowserStorage
				}
			}
		default:
			return nil, errBrowserStorage
		}
	}
	if last, err := decoder.Token(); err != nil || last != json.Delim('}') || len(seen) != 3 {
		return nil, errBrowserStorage
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errBrowserStorage
	}
	var extra [1]byte
	if n, err := input.Read(extra[:]); n != 0 || err != io.EOF {
		return nil, errBrowserStorage
	}
	return roots, nil
}

func validateBrowserStorageLocation(ctx context.Context, directory string, roots []string) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) == filepath.VolumeName(directory)+string(filepath.Separator) {
		return errBrowserStorage
	}
	if err := execution.CheckPrivateAncestry(directory); err != nil {
		return err
	}
	for _, root := range roots {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := execution.PrivateStateDisjointFromRoot(root, directory); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func prepareBrowserStorage(ctx context.Context, directory string, roots []string) error {
	if err := validateBrowserStorageLocation(ctx, directory, roots); err != nil {
		return err
	}
	for _, path := range []string{directory, filepath.Join(directory, "uploads")} {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := execution.EnsurePrivateDirectory(path); err != nil {
			return err
		}
	}
	// Existing journals and sidecars must already be private. New contents are
	// checked again by the host before use; private parent metadata alone is not
	// evidence of the permissions on a future file.
	for _, name := range []string{"dispatch.sqlite", "dispatch.sqlite-journal", "dispatch.sqlite-wal", "dispatch.sqlite-shm"} {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errBrowserStorage
		}
		if err = execution.CheckPrivatePath(path); err != nil {
			return err
		}
	}
	return checkBrowserUploadContents(ctx, filepath.Join(directory, "uploads"))
}

// Bound both directory width and depth. Retained/crashed allocations remain
// their ledger owner's responsibility; this checks metadata and privacy only.
func checkBrowserUploadContents(ctx context.Context, root string) error {
	entries, err := browserStorageEntries(root, 1024)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !browserStorageResourceID.MatchString(entry.Name()) {
			return errBrowserStorage
		}
		directory := filepath.Join(root, entry.Name())
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errBrowserStorage
		}
		if err = execution.CheckPrivatePath(directory); err != nil {
			return err
		}
		files, err := browserStorageEntries(directory, 1)
		if err != nil {
			return err
		}
		for _, file := range files {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			path := filepath.Join(directory, file.Name())
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				return errBrowserStorage
			}
			if err = execution.CheckPrivatePath(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func browserStorageEntries(path string, limit int) ([]os.DirEntry, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(limit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > limit {
		return nil, errBrowserStorage
	}
	return entries, nil
}
