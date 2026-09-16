package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSQLiteEveryConnectionIsDurableAndEnforcesReferences(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Unicode 日本語 # percent% space")
	if runtime.GOOS != "windows" {
		dir += "?mode=memory"
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "workspace.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id) ON DELETE CASCADE);
		INSERT INTO parent VALUES (1); INSERT INTO child VALUES (1, 1);`); err != nil {
		t.Fatal(err)
	}
	// Holding all four connections at once forces fresh physical connections.
	var connections []*sql.Conn
	for i := 0; i < 4; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		connections = append(connections, conn)
		for pragma, want := range map[string]int{"foreign_keys": 1, "synchronous": 2, "busy_timeout": 5000} {
			var got int
			if err := conn.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil || got != want {
				t.Fatalf("connection %d %s=%d; want %d: %v", i, pragma, got, want, err)
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO child VALUES (?, 999)", i+2); err == nil {
			t.Fatalf("connection %d accepted an orphan", i)
		}
	}
	if _, err := connections[3].ExecContext(ctx, "DELETE FROM parent WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := connections[2].QueryRowContext(ctx, "SELECT COUNT(*) FROM child").Scan(&count); err != nil || count != 0 {
		t.Fatalf("cascade count=%d: %v", count, err)
	}
	for _, conn := range connections {
		conn.Close()
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database opened at wrong path: %v", err)
	}
	reopened, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var integrity string
	if err := reopened.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q: %v", integrity, err)
	}
}

func TestWALResetFixVersionGate(t *testing.T) {
	for version, want := range map[string]bool{"3.50.4": false, "3.51.2": false, "3.51.3": true, "3.50.7": true, "3.44.6": true, "3.53.4": true, "unknown": false, "3.x.4": false} {
		if got := hasWALResetFix(version); got != want {
			t.Errorf("%s: got %v want %v", version, got, want)
		}
	}
}
