package rag

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
)

type collectionDestinationKey struct{}

func normalizedCollection(id string) string {
	if id == "" {
		return WorkspaceCollection
	}
	return id
}
func collectionDestination(ctx context.Context) string {
	id, _ := ctx.Value(collectionDestinationKey{}).(string)
	return normalizedCollection(id)
}
func collectionDocumentID(collection string, data []byte) string {
	collection = normalizedCollection(collection)
	if collection == WorkspaceCollection {
		return GenerateDocumentID(data)
	}
	digest := sha256.Sum256(data)
	return GenerateDocumentID([]byte("offgrid-collection-v1\x00" + collection + "\x00" + hex.EncodeToString(digest[:])))
}
func collectionAccess(q policyQuery, principal knowledgePrincipal, collection string, write bool) error {
	p, err := scanCollection(q.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, normalizedCollection(collection)))
	if err != nil || !principal.allows(p, write) {
		return ErrCollectionUnavailable
	}
	return nil
}
func importAccess(q policyQuery, owner, id string, write bool) (string, error) {
	var collection string
	err := q.QueryRow(`SELECT c.collection_id FROM import_collections c JOIN document_imports i ON i.id=c.import_id WHERE i.id=? AND i.owner=? AND i.schema_version=1 AND c.schema_version=1`, id, owner).Scan(&collection)
	if err != nil {
		return "", ErrImportNotFound
	}
	return collection, collectionAccess(q, knowledgePrincipal{actor: owner}, collection, write)
}
func bindPublication(tx *sql.Tx, doc *Document) error {
	collection := normalizedCollection(doc.collection)
	principal := knowledgePrincipal{sharedOnly: true}
	if doc.publicationActor != nil {
		principal = *doc.publicationActor
	}
	if err := collectionAccess(tx, principal, collection, true); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE document_collections SET collection_id=?,source_namespace=? WHERE document_id=? AND schema_version=1 AND revision=1`, collection, collection, doc.ID)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return ErrCollectionConflict
	}
	return nil
}
