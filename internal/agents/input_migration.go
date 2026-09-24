package agents

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Schema 3 introduced interruptions; schema 4 adds durable context and child
// graphs. Gate older binaries even though the SQLite table layout is stable.
func migrateTaskInputs(db *sql.DB, directory string) error {
	return migrateTaskVersion(db, directory, 2, taskSchemaVersion)
}

func migrateTaskVersion(db *sql.DB, directory string, from, to int) error {
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
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
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
	return tx.Commit()
}
