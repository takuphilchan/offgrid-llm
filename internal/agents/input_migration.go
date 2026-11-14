package agents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Schema 3 introduced interruptions; schema 4 adds durable context and child
// graphs; schema 5 adds correction epochs and stale dependency versions;
// schema 6 adds canonical conversation control; schema 7 adds user preferences.
// Schema 8 adds conversation inbox fences that older runners cannot enforce.
// Schema 9 retains work attempts and cumulative worker-session limits.
// Schema 10 retains child allocation settlement receipts across recovery.
// Schema 11 enforces pinned input-token reservations across runs and probes.
// Schema 12 enforces tool invocation reservations and settlement receipts.
// Schema 13 retains owned-process reservations and authenticated usage evidence.
// Older binaries must not open unsupported state.
// Schema 23 retains primary worker-question reviews and their pinned commands.
// Schema 24 pins worker admission, concurrency and allocation policy per work.
// Schema 25 seals canonical results and their source provenance independently of runs.
func migrateTaskInputs(db *sql.DB, directory string) error {
	return migrateTaskVersion(db, directory, 2, taskSchemaVersion)
}

func migrateTaskVersion(db *sql.DB, directory string, from, to int) (resultErr error) {
	if err := validateTaskActivityDatabase(db); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(directory, fmt.Sprintf("agent-schema-%d-backup-", from))
	if err != nil {
		return err
	}
	backup := filepath.Join(dir, taskDatabaseName)
	if _, err = db.Exec(`VACUUM INTO ?`, backup); err != nil {
		return fmt.Errorf("task input migration backup failed: %w", err)
	}
	if err = os.Chmod(backup, 0600); err != nil {
		return err
	}
	check, err := sql.Open("sqlite", backup)
	if err != nil {
		return err
	}
	verifyErr := validateTaskActivityDatabase(check)
	for _, table := range []string{"agent_tasks", "agent_task_events"} {
		if verifyErr != nil {
			break
		}
		var source, copied int
		if verifyErr = db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&source); verifyErr != nil {
			break
		}
		if verifyErr = check.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&copied); verifyErr != nil {
			break
		}
		if source != copied {
			verifyErr = fmt.Errorf("task input migration backup count mismatch")
		}
	}
	closeErr := check.Close()
	if verifyErr != nil {
		return verifyErr
	}
	if closeErr != nil {
		return closeErr
	}
	f, err := os.Open(backup)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, f)
	closeErr = f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	manifest, err := json.Marshal(map[string]any{"from": from, "to": to, "backup": filepath.Join(filepath.Base(dir), taskDatabaseName), "sha256": hex.EncodeToString(hash.Sum(nil)), "created_at": time.Now().UTC()})
	if err != nil {
		return err
	}
	// A dedicated connection prevents temporarily disabled foreign-key checks
	// from reaching another caller. Rebuild the referenced record table using
	// SQLite's documented create/copy/drop/rename migration sequence.
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer func() {
		_, restoreErr := conn.ExecContext(ctx, `PRAGMA foreign_keys=ON`)
		if restoreErr != nil {
			// Never return an unchecked connection to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			if resultErr == nil {
				resultErr = restoreErr
			}
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(controlSchema); err != nil {
		return err
	}
	if err = migrateControlRecordKinds(tx); err != nil {
		return err
	}
	if err = migrateControlResults(tx); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO agent_migrations(version,manifest) VALUES(?,?)`, to, manifest); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE agent_schema SET version=? WHERE singleton=1 AND version=?`, to, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("task schema changed during migration")
	}
	if err = checkControlForeignKeys(tx); err != nil {
		return err
	}
	return tx.Commit()
}
