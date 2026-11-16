package rag

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite"
)

// Preserve a consistent pre-upgrade database before the additive document
// migrations. Use SQLite's backup API so hidden rowids used by FTS stay intact.
// This is a separate retention domain, not a cache erased by document deletion.
func (s *SQLiteStore) snapshotBeforeDocumentUpgrade() error {
	var exists, upgraded int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='documents'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM pragma_table_info('chunks') WHERE name='embedding_v1') + (SELECT COUNT(*) FROM pragma_table_info('documents') WHERE name='extraction_revision')`).Scan(&upgraded); err != nil {
		return err
	}
	if upgraded == 2 {
		var policyTable, policyVersion int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_policy_migrations'`).Scan(&policyTable); err != nil {
			return err
		}
		if policyTable != 0 {
			if err := s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM knowledge_policy_migrations`).Scan(&policyVersion); err != nil {
				return err
			}
		}
		if policyVersion >= knowledgePolicySchema {
			var analyses int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='document_analyses'`).Scan(&analyses); err != nil {
				return err
			}
			if analyses != 0 {
				var synthesis int
				if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='document_synthesis_configs'`).Scan(&synthesis); err != nil {
					return err
				}
				var reviews int
				if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='document_synthesis_reviews'`).Scan(&reviews); err != nil {
					return err
				}
				var compositions int
				if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='document_compositions'`).Scan(&compositions); err != nil {
					return err
				}
				if synthesis != 0 && reviews != 0 && compositions != 0 {
					return nil
				}
			}
		}
	}
	file, err := os.CreateTemp(filepath.Dir(s.dbPath), ".before-document-upgrade-*.db")
	if err != nil {
		return fmt.Errorf("create pre-upgrade snapshot: %w", err)
	}
	name := file.Name()
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(name)
		}
	}()
	if err = file.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.Raw(func(raw any) (result error) {
		provider, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite backup unavailable")
		}
		backup, err := provider.NewBackup(name)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, backup.Finish()) }()
		for more := true; more; {
			if err = ctx.Err(); err != nil {
				return err
			}
			more, err = backup.Step(128)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pre-upgrade snapshot failed; source schema unchanged: %w", err)
	}
	copyDB, err := sql.Open("sqlite", name)
	if err != nil {
		return err
	}
	var integrity string
	err = copyDB.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity)
	err = errors.Join(err, copyDB.Close())
	if err != nil || integrity != "ok" {
		return fmt.Errorf("pre-upgrade snapshot validation failed: %v", err)
	}
	file, err = os.OpenFile(name, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	err = errors.Join(file.Sync(), file.Close())
	if err != nil {
		return err
	}
	complete = true
	return nil
}
