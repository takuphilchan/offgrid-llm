package rag

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Rebuild databases are derived caches, not independent workspace backups.
// Reconcile them against the authoritative live library after expiry and on
// startup. No filesystem trees are removed, and no source is republished.
// The engine lock serializes this with service mutations; offline rebuilding
// separately requires exclusive workspace ownership.
func (e *Engine) cleanRebuildStaging(ctx context.Context, live *SQLiteStore) error {
	root := filepath.Join(filepath.Dir(live.dbPath), "rebuild")
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := regularStagingPath(root, true); err != nil {
		return err
	}
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	entries, readErr := dir.ReadDir(1025)
	dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if len(entries) > 1024 {
		return ErrRetentionUnavailable
	}
	names := []string{}
	for _, entry := range entries {
		// Only directories generated from a pinned pipeline digest belong to us.
		if len(entry.Name()) != 64 || strings.ToLower(entry.Name()) != entry.Name() {
			continue
		}
		if _, err := hex.DecodeString(entry.Name()); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	start := sort.SearchStrings(names, e.rebuildRetentionCursor)
	for start < len(names) && names[start] <= e.rebuildRetentionCursor {
		start++
	}
	if start == len(names) {
		start = 0
	}
	// Round-robin prevents an old busy/large cache from starving newer caches.
	var firstError error
	for i := 0; i < len(names) && i < 8; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := names[(start+i)%len(names)]
		if err := cleanRebuildDatabase(ctx, filepath.Join(root, name), live.dbPath); err != nil && firstError == nil {
			firstError = err
		}
		e.rebuildRetentionCursor = name
	}
	return firstError
}

func regularStagingPath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (directory && !info.IsDir()) || (!directory && !info.Mode().IsRegular()) {
		return ErrRetentionUnavailable
	}
	return nil
}

func cleanRebuildDatabase(ctx context.Context, dir, livePath string) error {
	if err := regularStagingPath(dir, true); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		err := regularStagingPath(filepath.Join(dir, "rag.db"+suffix), false)
		if err != nil && !(suffix != "" && errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	stage, err := NewSQLiteStore(dir)
	if err != nil {
		return err
	}
	defer stage.Close()
	ids, err := staleRebuildDocuments(ctx, stage, livePath)
	if err != nil {
		return err
	}
	var firstError error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := stage.deleteDocumentContext(ctx, id); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func staleRebuildDocuments(ctx context.Context, stage *SQLiteStore, livePath string) ([]string, error) {
	conn, err := stage.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "ATTACH DATABASE ? AS current_library", livePath); err != nil {
		return nil, err
	}
	defer conn.ExecContext(context.Background(), "DETACH DATABASE current_library")
	// Do not purge an earlier version while the live document is held, or if
	// its current retention policy is missing/unknown. Unrelated live sources
	// and their resumable vectors remain untouched.
	rows, err := conn.QueryContext(ctx, `SELECT s.id FROM documents s
		LEFT JOIN current_library.documents d ON d.id=s.id
		LEFT JOIN current_library.document_retention r ON r.document_id=d.id
		WHERE d.id IS NULL OR (s.raw_content<>d.raw_content AND r.schema_version=1 AND r.revision>0 AND r.legal_hold=0)
		ORDER BY s.id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	return ids, err
}
