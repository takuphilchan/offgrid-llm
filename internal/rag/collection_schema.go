package rag

import "fmt"

const knowledgePolicySchema = 4

// Versioned, transactional policy foundation. This adds no document sharing or
// tenant-isolation claim: document bindings and enforcement must precede APIs.
func (s *SQLiteStore) initCollectionSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS knowledge_policy_migrations (version INTEGER PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS knowledge_collections (
			id TEXT PRIMARY KEY, schema_version INTEGER NOT NULL, revision INTEGER NOT NULL,
			name TEXT NOT NULL, owner TEXT NOT NULL, readers TEXT NOT NULL, editors TEXT NOT NULL
		)`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	var version int
	if err = tx.QueryRow(`SELECT COALESCE(MAX(version),0) FROM knowledge_policy_migrations`).Scan(&version); err != nil {
		return err
	}
	if version > knowledgePolicySchema {
		return fmt.Errorf("Knowledge policy schema requires a newer OffGrid version")
	}
	if version == 0 {
		if _, err = tx.Exec(`INSERT INTO knowledge_collections (` + collectionColumns + `) VALUES('workspace',1,1,'Workspace','', '[]','[]')`); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO knowledge_policy_migrations(version) VALUES(1)`); err != nil {
			return err
		}
	}
	if version < 2 {
		for _, statement := range []string{
			`CREATE TABLE document_collections (
				document_id TEXT PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
				collection_id TEXT NOT NULL REFERENCES knowledge_collections(id),
				schema_version INTEGER NOT NULL, revision INTEGER NOT NULL
			)`,
			`INSERT INTO document_collections SELECT id,'workspace',1,1 FROM documents`,
			`CREATE TRIGGER documents_collection_insert AFTER INSERT ON documents BEGIN
				INSERT INTO document_collections(document_id,collection_id,schema_version,revision) VALUES(new.id,'workspace',1,1);
			END`,
			`CREATE INDEX idx_document_collections_collection ON document_collections(collection_id)`,
			`INSERT INTO knowledge_policy_migrations(version) VALUES(2)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
	}
	if version < 3 {
		for _, statement := range []string{
			`ALTER TABLE document_collections ADD COLUMN source_namespace TEXT NOT NULL DEFAULT 'workspace'`,
			`CREATE TABLE import_collections (import_id TEXT PRIMARY KEY REFERENCES document_imports(id) ON DELETE CASCADE, collection_id TEXT NOT NULL REFERENCES knowledge_collections(id), schema_version INTEGER NOT NULL)`,
			`INSERT INTO import_collections SELECT id,'workspace',1 FROM document_imports`,
			`CREATE TRIGGER imports_collection_insert AFTER INSERT ON document_imports BEGIN INSERT INTO import_collections VALUES(new.id,'workspace',1); END`,
			`INSERT INTO knowledge_policy_migrations(version) VALUES(3)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
	}
	if version < 4 {
		for _, statement := range []string{
			// No cascade: INSERT OR REPLACE during reindexing must not erase a hold.
			`CREATE TABLE document_retention (document_id TEXT PRIMARY KEY, schema_version INTEGER NOT NULL, revision INTEGER NOT NULL, delete_after INTEGER NOT NULL DEFAULT 0, legal_hold INTEGER NOT NULL DEFAULT 0)`,
			`INSERT INTO document_retention SELECT id,1,1,0,0 FROM documents`,
			`CREATE TRIGGER documents_retention_insert AFTER INSERT ON documents BEGIN INSERT OR IGNORE INTO document_retention(document_id,schema_version,revision) VALUES(new.id,1,1); END`,
			`INSERT INTO knowledge_policy_migrations(version) VALUES(4)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return err
			}
		}
	}
	// Never recreate a missing/corrupt shared policy on restart: doing so could
	// turn a future migration or damaged record into an implicit grant.
	p, err := scanCollection(tx.QueryRow(`SELECT ` + collectionColumns + ` FROM knowledge_collections WHERE id='workspace'`))
	if err != nil || p.ID != WorkspaceCollection {
		return fmt.Errorf("Knowledge workspace policy is unavailable")
	}
	return tx.Commit()
}
