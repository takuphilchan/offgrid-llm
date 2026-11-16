package rag

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// IngestText ingests plain text content
func (e *Engine) IngestText(ctx context.Context, name, content string, metadata map[string]string) (*Document, error) {
	// User metadata is not an extractor receipt. Parser entry points attach
	// their own evidence; plain-text clients cannot assert reviewed coverage.
	retained := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if key != "extraction_coverage" {
			retained[key] = value
		}
	}
	return e.ingestText(ctx, name, content, retained, nil, "text/plain", nil)
}

func (e *Engine) ingestText(ctx context.Context, name, content string, metadata map[string]string, ranges []SourceRange, contentType string, original *OriginalSource, structures ...*document.Extraction) (*Document, error) {
	if err := e.ingestMu.acquire(ctx); err != nil {
		return nil, err
	}
	defer e.ingestMu.release()
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.enabled {
		return nil, fmt.Errorf("RAG is not enabled")
	}

	// Clean and validate content
	content = strings.TrimSpace(content)
	if len(content) < 10 {
		return nil, fmt.Errorf("content too short (minimum 10 characters)")
	}

	principal := principalFromContext(ctx)
	collection := collectionDestination(ctx)
	if store, ok := e.store.(*SQLiteStore); ok {
		store.mu.RLock()
		err := collectionAccess(store.db, principal, collection, true)
		store.mu.RUnlock()
		if err != nil {
			return nil, err
		}
	} else if collection != WorkspaceCollection {
		return nil, ErrCollectionUnavailable
	}
	docID := collectionDocumentID(collection, []byte(content))
	if original != nil {
		docID = collectionDocumentID(collection, original.Bytes)
	}

	// Check for duplicate
	existingDoc, err := e.store.GetDocument(docID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing document: %w", err)
	}
	if existingDoc != nil {
		// Deduplication must preserve the requested audience even when the actor
		// can read both collections. Metadata cannot select the destination.
		if store, ok := e.store.(*SQLiteStore); ok {
			store.mu.RLock()
			policy, _, policyErr := documentPolicy(store.db, docID)
			store.mu.RUnlock()
			if policyErr != nil || policy.ID != collection {
				return nil, ErrCollectionUnavailable
			}
		}
		if err := e.documentAccess(principal, docID, true); err != nil {
			return nil, err
		}
		if original != nil && original.importRef != nil {
			// Same original bytes do not imply the same reviewed evidence. Never
			// silently substitute an existing library revision for the selected one.
			if original.revisions != nil && (len(structures) == 0 || existingDoc.ExtractionRevision != structures[0].ID) {
				return nil, ErrExtractionConflict
			}
			if store, ok := e.store.(*SQLiteStore); ok {
				if err := store.linkExistingImport(existingDoc, original); err != nil {
					return nil, err
				}
				return existingDoc, nil
			}
		}
		return nil, fmt.Errorf("document with identical content already exists: %s", existingDoc.Name)
	}

	doc := &Document{
		collection:       collection,
		publicationActor: &principal,
		original:         original,
		OriginalRetained: original != nil,
		SourceRanges:     append([]SourceRange(nil), ranges...),
		ID:               docID,
		Name:             name,
		ContentType:      contentType,
		Size:             int64(len(content)),
		ContentHash:      GenerateContentHash(content),
		IndexStatus:      "indexing",
		SourceRetained:   true,
		RawContent:       content,
		Metadata:         metadata,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Use auto-tuned chunking if enabled
	if len(structures) > 0 {
		doc.Extraction = structures[0]
	}
	if err := doc.ensureExtraction(); err != nil {
		return nil, err
	}
	var chunker *Chunker
	if e.autoTuneChunks {
		opts, analysis := AutoTuneChunkingOptions(content)
		chunker = NewChunker(opts)
		log.Printf("[RAG] Auto-tuned chunking for '%s': type=%s, chunk_size=%d, reasoning=%s",
			name, analysis.DocumentType, opts.ChunkSize, analysis.Reasoning)
		// Store analysis in metadata
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}
		doc.Metadata["doc_type"] = analysis.DocumentType
		doc.Metadata["chunk_size"] = fmt.Sprintf("%d", opts.ChunkSize)
		doc.Metadata["auto_tuned"] = "true"
	} else {
		chunker = e.chunker
	}

	// Chunk the document
	chunks, err := chunker.chunkDocument(doc)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from content")
	}
	doc.ChunkCount = len(chunks)

	// Generate embeddings for all chunks in batches
	const batchSize = 32
	allEmbeddings := make([][]float32, 0, len(chunks))

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		texts := make([]string, end-i)
		for j, chunk := range chunks[i:end] {
			texts[j] = chunk.Content
		}

		embeddings, err := e.generateEmbeddings(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings for batch %d: %w", i/batchSize, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}
	if len(allEmbeddings) != len(chunks) || len(allEmbeddings) == 0 {
		return nil, fmt.Errorf("embedding response count mismatch: got %d for %d chunks", len(allEmbeddings), len(chunks))
	}
	embeddingDim := len(allEmbeddings[0])
	if embeddingDim == 0 {
		return nil, fmt.Errorf("embedding model returned an empty vector")
	}
	for i, embedding := range allEmbeddings {
		if len(embedding) != embeddingDim {
			return nil, fmt.Errorf("embedding %d has dimension %d, expected %d", i, len(embedding), embeddingDim)
		}
		chunks[i].Embedding = append([]float32(nil), embedding...)
		chunks[i].CreatedAt = time.Now().UTC()
	}
	doc.IndexStatus = "ready"
	doc.IndexedAt = time.Now().UTC()

	indexMetadata := IndexMetadata{
		EmbeddingIdentity: e.embeddingIdentity,
		SchemaVersion:     IndexSchemaVersion,
		EmbeddingModel:    e.embeddingModel,
		EmbeddingDim:      embeddingDim,
		ChunkerVersion:    chunkerVersion,
		ParserVersion:     parserVersion,
		UpdatedAt:         time.Now().UTC(),
	}
	if metadataStore, ok := e.store.(indexMetadataStore); ok {
		existing, found, err := metadataStore.GetIndexMetadata()
		if err != nil {
			return nil, fmt.Errorf("read index metadata: %w", err)
		}
		if found && (existing.EmbeddingModel != indexMetadata.EmbeddingModel || existing.EmbeddingDim != embeddingDim) {
			return nil, fmt.Errorf("embedding identity mismatch: index uses %s/%d, model returned %s/%d",
				existing.EmbeddingModel, existing.EmbeddingDim, indexMetadata.EmbeddingModel, embeddingDim)
		}
	}

	// Store the document and all chunks as one logical operation.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store, ok := e.store.(*SQLiteStore); ok {
		if err := store.PublishDocument(doc, chunks, allEmbeddings, indexMetadata, false); err != nil {
			return nil, fmt.Errorf("failed to publish document index: %w", err)
		}
		return doc, nil
	} else if store, ok := e.store.(batchStore); ok {
		if err := store.AddDocumentWithChunks(doc, chunks, allEmbeddings); err != nil {
			return nil, fmt.Errorf("failed to atomically store document: %w", err)
		}
	} else {
		if err := e.store.AddDocument(doc); err != nil {
			return nil, fmt.Errorf("failed to store document: %w", err)
		}
		for i, chunk := range chunks {
			if err := e.store.AddChunk(chunk, allEmbeddings[i]); err != nil {
				_ = e.store.DeleteDocument(doc.ID)
				return nil, fmt.Errorf("failed to store chunk %d: %w", i, err)
			}
		}
	}
	if metadataStore, ok := e.store.(indexMetadataStore); ok {
		if err := metadataStore.SetIndexMetadata(indexMetadata); err != nil {
			_ = e.store.DeleteDocument(doc.ID)
			return nil, fmt.Errorf("persist index metadata: %w", err)
		}
	}

	// Persist to disk (No longer needed with SQLite, but keeping for backward compatibility if we had other stores)
	// if err := e.saveToDisk(); err != nil {
	// 	log.Printf("Warning: failed to save RAG data to disk: %v", err)
	// }

	log.Printf("[RAG] Ingested document '%s' with %d chunks (%d embeddings)", name, len(chunks), len(allEmbeddings))
	return doc, nil
}
