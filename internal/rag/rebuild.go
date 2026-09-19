package rag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// RebuildKnowledgeOffline requires exclusive workspace ownership. It never
// removes sources or the old index; completed staging documents survive a
// cancellation/crash and are reused on an explicit retry with the same pipeline.
// An explicit local runtime prevents network installation during maintenance.
func RebuildKnowledgeOffline(ctx context.Context, dataDir, modelID, modelPath, runtimePath string, progress func(int, int)) error {
	if strings.TrimSpace(modelID) == "" || runtimePath == "" {
		return fmt.Errorf("model ID and an explicit installed runtime are required")
	}
	owner, err := storage.AcquireOwnership(dataDir)
	if err != nil {
		return err
	}
	defer owner.Close()
	worker := inference.NewEmbeddingEngine()
	defer worker.Unload()
	opts := inference.DefaultEmbeddingOptions()
	opts.RuntimePath = runtimePath
	if err := worker.Load(ctx, modelPath, opts); err != nil {
		return err
	}
	engine := NewEngine(worker, dataDir)
	defer engine.Close()
	if err := engine.StorageError(); err != nil {
		return err
	}
	engine.embeddingModel = modelID
	engine.embeddingIdentity, _ = worker.GetModelInfo()["identity"].(string)
	if engine.embeddingIdentity == "" {
		return fmt.Errorf("runtime has no verified embedding identity")
	}
	return engine.rebuildOffline(ctx, progress)
}

func (e *Engine) rebuildOffline(ctx context.Context, progress func(int, int)) error {
	store, ok := e.store.(*SQLiteStore)
	if !ok {
		return fmt.Errorf("atomic rebuild requires SQLite storage")
	}
	docs, err := store.ListDocuments()
	if err != nil {
		return err
	}
	// Fail before staging if any original text is unavailable. Never silently
	// drop legacy documents merely to make the replacement appear successful.
	wanted := make(map[string]bool, len(docs))
	for _, doc := range docs {
		if !doc.SourceRetained {
			return fmt.Errorf("source is not retained for document %q (%s); recover/re-import its original source before rebuilding", doc.Name, doc.ID)
		}
		wanted[doc.ID] = true
	}
	identity := fmt.Sprintf("%s:%s:%s:%d", e.embeddingIdentity, chunkerVersion, parserVersion, IndexSchemaVersion)
	digest := sha256.Sum256([]byte(identity))
	stage, err := NewSQLiteStore(filepath.Join(e.dataDir, "rag", "rebuild", fmt.Sprintf("%x", digest)))
	if err != nil {
		return err
	}
	defer stage.Close()
	staged, err := stage.ListDocuments()
	if err != nil {
		return err
	}
	for _, doc := range staged {
		if !wanted[doc.ID] {
			if err := stage.DeleteDocument(doc.ID); err != nil {
				return err
			}
		}
	}
	metadata := IndexMetadata{SchemaVersion: IndexSchemaVersion, EmbeddingModel: e.embeddingModel, EmbeddingIdentity: e.embeddingIdentity,
		EmbeddingDim: e.embeddingEngine.GetDimensions(), ChunkerVersion: chunkerVersion, ParserVersion: parserVersion, UpdatedAt: time.Now().UTC()}
	if err := stage.SetIndexMetadata(metadata); err != nil {
		return err
	}
	for i, summary := range docs {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc, err := store.GetDocument(summary.ID)
		if err != nil {
			return err
		}
		if doc == nil {
			return fmt.Errorf("document disappeared during exclusive rebuild")
		}
		previous, err := stage.GetDocument(doc.ID)
		if err != nil {
			return err
		}
		if previous != nil && previous.RawContent == doc.RawContent && previous.ContentHash == GenerateContentHash(doc.RawContent) && previous.IndexStatus == "ready" {
			if progress != nil {
				progress(i+1, len(docs))
			}
			continue
		}
		chunks, vectors, _, err := e.buildDocumentIndex(ctx, doc, doc.RawContent)
		if err != nil {
			return fmt.Errorf("rebuild document %s: %w", doc.ID, err)
		}
		doc.ChunkCount = len(chunks)
		doc.ContentHash = GenerateContentHash(doc.RawContent)
		doc.IndexStatus = "ready"
		doc.LastError = ""
		doc.IndexedAt = time.Now().UTC()
		if previous == nil {
			err = stage.AddDocumentWithChunks(doc, chunks, vectors)
		} else {
			err = stage.ReplaceDocumentWithChunks(doc, chunks, vectors)
		}
		if err != nil {
			return err
		}
		if progress != nil {
			progress(i+1, len(docs))
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return store.publishStagedIndex(ctx, stage.dbPath)
}

// Only the main database is written. All replacement vectors and pipeline
// metadata are committed together; a failure leaves the original index intact.
func (s *SQLiteStore) publishStagedIndex(ctx context.Context, stagePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "ATTACH DATABASE ? AS rebuilt", stagePath); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "DETACH DATABASE rebuilt")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var schema, dimensions int
	var identity string
	if err := tx.QueryRowContext(ctx, `SELECT schema_version,embedding_dimension,embedding_identity FROM rebuilt.index_metadata WHERE id=1`).Scan(&schema, &dimensions, &identity); err != nil {
		return err
	}
	if schema != IndexSchemaVersion || dimensions <= 0 || identity == "" {
		return fmt.Errorf("staged index lacks verified pipeline metadata")
	}
	var mismatch int
	err = tx.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM documents d LEFT JOIN rebuilt.documents r ON d.id=r.id WHERE r.id IS NULL OR d.raw_content != r.raw_content)
		+ (SELECT COUNT(*) FROM rebuilt.documents r LEFT JOIN documents d ON r.id=d.id WHERE d.id IS NULL)
		+ (SELECT COUNT(*) FROM rebuilt.documents r WHERE r.index_status!='ready' OR r.chunk_count<=0 OR r.chunk_count != (SELECT COUNT(*) FROM rebuilt.chunks c WHERE c.document_id=r.id))
		+ (SELECT COUNT(*) FROM rebuilt.chunks WHERE json_array_length(embedding) != ?)`, dimensions).Scan(&mismatch)
	if err != nil {
		return err
	}
	if mismatch != 0 {
		return fmt.Errorf("staged index does not match retained sources; original index unchanged")
	}
	for _, query := range []string{
		`DELETE FROM chunks`,
		`INSERT INTO chunks (id,document_id,content,chunk_index,start_char,end_char,page,section,content_hash,embedding,created_at)
		 SELECT id,document_id,content,chunk_index,start_char,end_char,page,section,content_hash,embedding,created_at FROM rebuilt.chunks`,
		`UPDATE documents SET chunk_count=(SELECT chunk_count FROM rebuilt.documents r WHERE r.id=documents.id),
		 content_hash=(SELECT content_hash FROM rebuilt.documents r WHERE r.id=documents.id),
		 indexed_at=(SELECT indexed_at FROM rebuilt.documents r WHERE r.id=documents.id),index_status='ready',last_error=''`,
		`DELETE FROM index_metadata`,
		`INSERT INTO index_metadata (id,schema_version,embedding_model,embedding_dimension,chunker_version,parser_version,updated_at,embedding_identity)
		 SELECT id,schema_version,embedding_model,embedding_dimension,chunker_version,parser_version,updated_at,embedding_identity FROM rebuilt.index_metadata`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}
