package rag

import "testing"

func TestStoreImplementation(t *testing.T) {
	var _ Store = (*VectorStore)(nil)
	var _ Store = (*SQLiteStore)(nil)
}
