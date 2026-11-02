package rag

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

const maxOriginalBytes = 50 << 20
const maxLibraryOriginalBytes = 2 << 30
const originalSchema = 1

var ErrOriginalUnavailable = errors.New("original document unavailable")

// OriginalSource is never included in a document/list/search JSON response.
// Its authority is the enclosing Knowledge workspace's existing policy; it is
// not a conversation-private attachment or a cross-workspace deduplication key.
type OriginalSource struct {
	importRef *importLease
	Name      string
	MediaType string
	SHA256    string
	Bytes     []byte
}

func (e *Engine) IngestOriginal(ctx context.Context, name string, data []byte, result *ParseResult, metadata map[string]string) (*Document, error) {
	return e.ingestOriginal(ctx, name, data, result, metadata, nil)
}

func (e *Engine) ingestOriginal(ctx context.Context, name string, data []byte, result *ParseResult, metadata map[string]string, lease *importLease) (*Document, error) {
	content, ranges, err := parsedSourceRanges(result)
	if err != nil {
		return nil, err
	}
	if _, ok := e.store.(*SQLiteStore); !ok {
		return nil, fmt.Errorf("durable original storage unavailable")
	}
	if len(data) == 0 || len(data) > maxOriginalBytes {
		return nil, &ParseError{Code: "document_limit", Message: "Documents must contain between 1 byte and 50 MiB."}
	}
	digest := sha256.Sum256(data)
	original := &OriginalSource{importRef: lease, Name: name, MediaType: result.ContentType, SHA256: hex.EncodeToString(digest[:]), Bytes: append([]byte(nil), data...)}
	for _, r := range ranges {
		if r.SHA256 != original.SHA256 {
			return nil, fmt.Errorf("extraction does not match original source")
		}
	}
	return e.ingestText(ctx, name, content, metadata, ranges, result.ContentType, original)
}

func (e *Engine) Original(id string) (*OriginalSource, error) {
	return e.viewForContext(context.Background()).Original(id)
}

func (s *SQLiteStore) storeOriginal(tx *sql.Tx, doc *Document) error {
	original := doc.original
	if original == nil {
		return nil
	}
	digest := sha256.Sum256(original.Bytes)
	if len(original.Bytes) == 0 || len(original.Bytes) > maxOriginalBytes || hex.EncodeToString(digest[:]) != original.SHA256 || doc.ID != collectionDocumentID(doc.collection, original.Bytes) {
		return fmt.Errorf("invalid original source identity")
	}
	if err := checkOriginalQuota(tx, s.originalQuota, int64(len(original.Bytes))); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO document_originals(document_id,schema_version,source_sha256,byte_size,media_type,original_name,content) VALUES(?,?,?,?,?,?,?)`, doc.ID, originalSchema, original.SHA256, len(original.Bytes), original.MediaType, original.Name, original.Bytes)
	return err
}

func (s *SQLiteStore) original(id string) (*OriginalSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var source OriginalSource
	var namespace string
	if err := s.db.QueryRow(`SELECT source_namespace FROM document_collections WHERE document_id=? AND schema_version=1`, id).Scan(&namespace); err != nil || namespace == "" {
		return nil, ErrOriginalUnavailable
	}
	var version, size, actual int
	// Validate size/version before materializing a stored BLOB; unknown future
	// records remain in the database but cannot be interpreted as current bytes.
	err := s.db.QueryRow(`SELECT schema_version,byte_size,LENGTH(content) FROM document_originals WHERE document_id=?`, id).Scan(&version, &size, &actual)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOriginalUnavailable
	}
	if err != nil {
		return nil, err
	}
	if version != originalSchema || size < 1 || size > maxOriginalBytes || size != actual {
		return nil, ErrOriginalUnavailable
	}
	err = s.db.QueryRow(`SELECT original_name,media_type,source_sha256,content FROM document_originals WHERE document_id=? AND schema_version=? AND byte_size=? AND LENGTH(content)=?`, id, originalSchema, size, size).Scan(&source.Name, &source.MediaType, &source.SHA256, &source.Bytes)
	if err != nil {
		return nil, errors.Join(ErrOriginalUnavailable, err)
	}
	digest := sha256.Sum256(source.Bytes)
	if hex.EncodeToString(digest[:]) != source.SHA256 || collectionDocumentID(namespace, source.Bytes) != id {
		return nil, ErrOriginalUnavailable
	}
	return &source, nil
}
