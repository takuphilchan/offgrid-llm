package rag

import (
	"database/sql"
	"errors"
	"fmt"
)

// PublishDocument commits source, vectors and their interpretation together.
// A metadata failure rolls back the new document or preserves the old index.
func (s *SQLiteStore) PublishDocument(doc *Document, chunks []*Chunk, embeddings [][]float32, index IndexMetadata, replace bool) error {
	if replace {
		return s.replaceDocumentWithChunks(doc, chunks, embeddings, &index)
	}
	return s.addDocumentWithChunks(doc, chunks, embeddings, &index)
}

func storePublicationIdentity(tx *sql.Tx, index IndexMetadata) error {
	if index.SchemaVersion != IndexSchemaVersion || index.EmbeddingModel == "" || index.EmbeddingDim < 1 || index.ChunkerVersion == "" || index.ParserVersion == "" || index.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid index publication identity")
	}
	var prior IndexMetadata
	err := tx.QueryRow(`SELECT embedding_model,embedding_dimension,embedding_identity FROM index_metadata WHERE id=1`).Scan(&prior.EmbeddingModel, &prior.EmbeddingDim, &prior.EmbeddingIdentity)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && (prior.EmbeddingModel != index.EmbeddingModel || prior.EmbeddingDim != index.EmbeddingDim || (prior.EmbeddingIdentity != "" && prior.EmbeddingIdentity != index.EmbeddingIdentity)) {
		return fmt.Errorf("index publication identity conflicts with the existing index")
	}
	_, err = tx.Exec(`INSERT INTO index_metadata(id,schema_version,embedding_model,embedding_dimension,chunker_version,parser_version,updated_at,embedding_identity)
		VALUES(1,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		schema_version=excluded.schema_version,embedding_model=excluded.embedding_model,
		embedding_dimension=excluded.embedding_dimension,chunker_version=excluded.chunker_version,
		parser_version=excluded.parser_version,updated_at=excluded.updated_at,embedding_identity=excluded.embedding_identity`,
		index.SchemaVersion, index.EmbeddingModel, index.EmbeddingDim, index.ChunkerVersion, index.ParserVersion, index.UpdatedAt, index.EmbeddingIdentity)
	return err
}
