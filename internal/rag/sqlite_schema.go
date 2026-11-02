package rag

import (
	"fmt"
	"strings"
)

// initSchema initializes the database schema
func (s *SQLiteStore) initSchema() error {
	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			content_type TEXT,
			size INTEGER,
			chunk_count INTEGER,
			content_hash TEXT DEFAULT '',
			index_status TEXT DEFAULT 'ready',
			last_error TEXT DEFAULT '',
			indexed_at DATETIME,
			raw_content TEXT DEFAULT '',
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content TEXT NOT NULL,
			chunk_index INTEGER,
			start_char INTEGER,
			end_char INTEGER,
			page INTEGER DEFAULT 0,
			section TEXT DEFAULT '',
			content_hash TEXT DEFAULT '',
			embedding BLOB, -- Stored as JSON array of floats for now (simple)
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(document_id);`,
		`CREATE TABLE IF NOT EXISTS document_originals (
			document_id TEXT PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
			schema_version INTEGER NOT NULL,
			source_sha256 TEXT NOT NULL,
			byte_size INTEGER NOT NULL CHECK(byte_size > 0 AND byte_size <= 52428800),
			media_type TEXT NOT NULL,
			original_name TEXT NOT NULL,
			content BLOB NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS index_metadata (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			schema_version INTEGER NOT NULL,
			embedding_model TEXT NOT NULL,
			embedding_dimension INTEGER NOT NULL,
			chunker_version TEXT NOT NULL,
			parser_version TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS document_imports (
			id TEXT PRIMARY KEY, schema_version INTEGER NOT NULL,
			owner TEXT NOT NULL, request_key TEXT NOT NULL, name TEXT NOT NULL,
			source_sha256 TEXT NOT NULL, byte_size INTEGER NOT NULL CHECK(byte_size>0 AND byte_size<=52428800),
			content BLOB, state TEXT NOT NULL, attempt INTEGER NOT NULL DEFAULT 0,
			error_code TEXT NOT NULL DEFAULT '', document_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
			UNIQUE(owner,request_key)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_document_imports_owner ON document_imports(owner,created_at);`,
		// FTS5 virtual table for full-text search (hybrid search support)
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
			chunk_id,
			content,
			content='chunks',
			content_rowid='rowid'
		);`,
		// Triggers to keep FTS index in sync
		`CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
			INSERT INTO chunks_fts(rowid, chunk_id, content) VALUES (new.rowid, new.id, new.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
			INSERT INTO chunks_fts(chunks_fts, rowid, chunk_id, content) VALUES('delete', old.rowid, old.id, old.content);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
			INSERT INTO chunks_fts(chunks_fts, rowid, chunk_id, content) VALUES('delete', old.rowid, old.id, old.content);
			INSERT INTO chunks_fts(rowid, chunk_id, content) VALUES (new.rowid, new.id, new.content);
		END;`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			// Ignore errors for FTS5 if not supported (older SQLite)
			if !isFTS5Error(err) {
				return fmt.Errorf("failed to execute init query: %w", err)
			}
		}
	}

	// Add locator columns for indexes created before schema version 2. SQLite
	// does not support ADD COLUMN IF NOT EXISTS on all supported versions, so
	// duplicate-column errors are intentionally ignored.
	for _, migration := range []string{
		`ALTER TABLE document_imports ADD COLUMN extraction_json TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE index_metadata ADD COLUMN embedding_identity TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN content_hash TEXT DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN index_status TEXT DEFAULT 'ready'`,
		`ALTER TABLE documents ADD COLUMN last_error TEXT DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN indexed_at DATETIME`,
		`ALTER TABLE documents ADD COLUMN raw_content TEXT DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN source_ranges TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE chunks ADD COLUMN page INTEGER DEFAULT 0`,
		`ALTER TABLE chunks ADD COLUMN section TEXT DEFAULT ''`,
		`ALTER TABLE chunks ADD COLUMN content_hash TEXT DEFAULT ''`,
	} {
		if _, err := s.db.Exec(migration); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("failed to migrate RAG schema: %w", err)
		}
	}

	return s.initCollectionSchema()
}

// isFTS5Error checks if an error is related to FTS5 not being available
func isFTS5Error(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "no such module: fts5" ||
		errStr == "unknown virtual table: fts5" ||
		errStr == "no such table: chunks_fts"
}
