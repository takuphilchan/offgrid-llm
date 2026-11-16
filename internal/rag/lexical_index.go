package rag

import (
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/textsearch"
)

// This additive, derived index does not alter retained text or embedding identity.
// All chunk writers supply search_terms; triggers keep publication atomic.
func (s *SQLiteStore) initLexicalIndex() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunk_terms USING fts5(terms)`,
		`CREATE TRIGGER IF NOT EXISTS chunk_terms_insert AFTER INSERT ON chunks BEGIN INSERT INTO chunk_terms(rowid,terms) VALUES(new.rowid,new.search_terms); END`,
		`CREATE TRIGGER IF NOT EXISTS chunk_terms_delete AFTER DELETE ON chunks BEGIN DELETE FROM chunk_terms WHERE rowid=old.rowid; END`,
		`CREATE TRIGGER IF NOT EXISTS chunk_terms_update AFTER UPDATE OF search_terms ON chunks BEGIN DELETE FROM chunk_terms WHERE rowid=old.rowid; INSERT INTO chunk_terms(rowid,terms) VALUES(new.rowid,new.search_terms); END`,
	} {
		if _, err = tx.Exec(q); err != nil {
			return fmt.Errorf("initialize literal search: %w", err)
		}
	}
	// Bound migration batches. Missing derived rows are recoverable without
	// reopening source files or calling an embedding/model provider.
	var after int64
	for {
		rows, err := tx.Query(`SELECT c.rowid,c.content FROM chunks c LEFT JOIN chunk_terms f ON f.rowid=c.rowid WHERE c.rowid>? AND (f.rowid IS NULL OR c.search_terms='') ORDER BY c.rowid LIMIT 128`, after)
		if err != nil {
			return err
		}
		type item struct {
			id   int64
			text string
		}
		var batch []item
		for rows.Next() {
			var x item
			if err := rows.Scan(&x.id, &x.text); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, x)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, x := range batch {
			if _, err = tx.Exec(`UPDATE chunks SET search_terms=? WHERE rowid=?`, textsearch.IndexText(x.text), x.id); err != nil {
				return err
			}
			after = x.id
		}
	}
	return tx.Commit()
}
