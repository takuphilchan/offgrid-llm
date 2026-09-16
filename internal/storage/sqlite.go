// Package storage defines the connection policy for durable local databases.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// OpenSQLite opens a local file using WAL, full commit durability, and foreign
// keys on EVERY pooled connection. Callers own schema and lifecycle. This is not
// a service ownership lock or a backup mechanism. Never use a network drive.
func OpenSQLite(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	filename := filepath.ToSlash(abs)
	if strings.HasPrefix(filename, "//") {
		return nil, fmt.Errorf("SQLite workspace storage must be on a local filesystem, not a UNC share")
	}
	// file:///C:/... on Windows; file:///... on Unix. Encoding the path
	// separately prevents '#' and '?' in Unix filenames becoming DSN options.
	if !strings.HasPrefix(filename, "/") {
		filename = "/" + filename
	}
	query := url.Values{}
	for _, pragma := range []string{"busy_timeout(5000)", "foreign_keys(1)", "synchronous(FULL)"} {
		query.Add("_pragma", pragma)
	}
	dsn := (&url.URL{Scheme: "file", Path: filename, RawQuery: query.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var version string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		db.Close()
		return nil, fmt.Errorf("check SQLite runtime: %w", err)
	}
	if !hasWALResetFix(version) {
		db.Close()
		return nil, fmt.Errorf("SQLite %s lacks the required WAL-reset fix; update OffGrid before opening workspace storage", version)
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable SQLite WAL: %w", err)
	}
	if mode != "wal" {
		db.Close()
		return nil, fmt.Errorf("SQLite WAL is unavailable on this storage")
	}
	return db, nil
}

func hasWALResetFix(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	var numbers [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return false
		}
		numbers[i] = n
	}
	// https://sqlite.org/wal.html#walresetbug: current releases and documented
	// backports. Pinning the driver remains necessary; this prevents regressions.
	return numbers[0] == 3 && (numbers[1] > 51 || numbers[1] == 51 && numbers[2] >= 3 || numbers[1] == 50 && numbers[2] >= 7 || numbers[1] == 44 && numbers[2] >= 6)
}
