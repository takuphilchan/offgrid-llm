package rag

func (s *SQLiteStore) initExtractionSchema() error {
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS document_extractions (
		 document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		 revision TEXT NOT NULL, schema_version INTEGER NOT NULL,
		 source_sha256 TEXT NOT NULL, raw_text TEXT NOT NULL,
		 graph_json TEXT NOT NULL, source_ranges TEXT NOT NULL, metadata TEXT NOT NULL,
		 storage_bytes INTEGER NOT NULL DEFAULT 0,
		 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		 PRIMARY KEY(document_id,revision))`,
		`CREATE TABLE IF NOT EXISTS document_blocks (
		 document_id TEXT NOT NULL, revision TEXT NOT NULL, block_id TEXT NOT NULL,
		 ordinal INTEGER NOT NULL, unit_id TEXT NOT NULL, kind TEXT NOT NULL,
		 start_byte INTEGER NOT NULL, end_byte INTEGER NOT NULL, block_json TEXT NOT NULL,
		 PRIMARY KEY(document_id,revision,block_id),
		 FOREIGN KEY(document_id,revision) REFERENCES document_extractions(document_id,revision) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS document_relationships (
		 document_id TEXT NOT NULL, revision TEXT NOT NULL, ordinal INTEGER NOT NULL,
		 from_id TEXT NOT NULL, to_id TEXT NOT NULL, kind TEXT NOT NULL,
		 PRIMARY KEY(document_id,revision,ordinal),
		 FOREIGN KEY(document_id,revision,from_id) REFERENCES document_blocks(document_id,revision,block_id) ON DELETE CASCADE,
		 FOREIGN KEY(document_id,revision,to_id) REFERENCES document_blocks(document_id,revision,block_id) ON DELETE CASCADE)`,
		`CREATE INDEX IF NOT EXISTS document_blocks_order ON document_blocks(document_id,revision,ordinal)`,
		`CREATE INDEX IF NOT EXISTS document_extractions_source ON document_extractions(document_id,source_sha256)`,
		`CREATE INDEX IF NOT EXISTS chunks_document_hash ON chunks(document_id,content_hash)`,
	} {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
