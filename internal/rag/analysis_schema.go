package rag

func (s *SQLiteStore) initAnalysisSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS document_analyses (
		 id TEXT PRIMARY KEY, schema_version INTEGER NOT NULL, owner TEXT NOT NULL,
		 request_key TEXT NOT NULL, plan_json TEXT NOT NULL, pairs_json TEXT NOT NULL,
		 state TEXT NOT NULL, attempt INTEGER NOT NULL DEFAULT 0,
		 completed_steps INTEGER NOT NULL DEFAULT 0, completed_units INTEGER NOT NULL DEFAULT 0,
		 review_required INTEGER NOT NULL DEFAULT 0,
		 error_code TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		 UNIQUE(owner,request_key))`,
		`CREATE TABLE IF NOT EXISTS document_analysis_sources (
		 analysis_id TEXT NOT NULL REFERENCES document_analyses(id) ON DELETE CASCADE,
		 ordinal INTEGER NOT NULL, document_id TEXT NOT NULL, revision TEXT NOT NULL,
		 PRIMARY KEY(analysis_id,ordinal),
		 FOREIGN KEY(document_id,revision) REFERENCES document_extractions(document_id,revision) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS document_analysis_steps (
		 analysis_id TEXT NOT NULL REFERENCES document_analyses(id) ON DELETE CASCADE,
		 ordinal INTEGER NOT NULL, result_json TEXT NOT NULL, storage_bytes INTEGER NOT NULL,
		 PRIMARY KEY(analysis_id,ordinal))`,
		`CREATE INDEX IF NOT EXISTS document_analyses_owner ON document_analyses(owner,created_at)`,
		`CREATE TABLE IF NOT EXISTS document_synthesis_configs (
		 analysis_id TEXT PRIMARY KEY REFERENCES document_analyses(id) ON DELETE CASCADE,
		 model_json TEXT NOT NULL, primary_bytes INTEGER NOT NULL,
		 unit_cursor INTEGER NOT NULL DEFAULT 0, next_byte INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS document_analysis_source_identity ON document_analysis_sources(document_id,revision)`,
		`CREATE TABLE IF NOT EXISTS document_synthesis_reviews (
		 analysis_id TEXT NOT NULL, ordinal INTEGER NOT NULL, revision INTEGER NOT NULL,
		 review_json TEXT NOT NULL, storage_bytes INTEGER NOT NULL,
		 PRIMARY KEY(analysis_id,ordinal,revision),
		 FOREIGN KEY(analysis_id,ordinal) REFERENCES document_analysis_steps(analysis_id,ordinal) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS document_compositions (
		 id TEXT PRIMARY KEY, analysis_id TEXT NOT NULL REFERENCES document_analyses(id) ON DELETE CASCADE,
		 owner TEXT NOT NULL, request_key TEXT NOT NULL, plan_json TEXT NOT NULL, model_json TEXT NOT NULL,
		 review_snapshot_json TEXT NOT NULL, state TEXT NOT NULL, attempt INTEGER NOT NULL DEFAULT 0,
		 completed_steps INTEGER NOT NULL DEFAULT 0,error_code TEXT NOT NULL DEFAULT '',
		 created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, UNIQUE(owner,request_key))`,
		`CREATE TABLE IF NOT EXISTS document_composition_steps (
		 composition_id TEXT NOT NULL REFERENCES document_compositions(id) ON DELETE CASCADE,
		 ordinal INTEGER NOT NULL,receipt_json TEXT NOT NULL,raw_response BLOB NOT NULL,
		 storage_bytes INTEGER NOT NULL,PRIMARY KEY(composition_id,ordinal))`,
		`CREATE INDEX IF NOT EXISTS document_compositions_owner ON document_compositions(owner,created_at)`,
		// A result involving multiple sources cannot outlive deletion of any one
		// source. Existing source legal-hold enforcement runs before that deletion.
		`CREATE TRIGGER IF NOT EXISTS document_analysis_source_delete AFTER DELETE ON document_analysis_sources BEGIN
		 DELETE FROM document_analyses WHERE id=old.analysis_id;
		 END`,
	} {
		if _, err = tx.Exec(query); err != nil {
			return err
		}
	}
	return tx.Commit()
}
