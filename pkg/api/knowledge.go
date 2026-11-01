package api

// KnowledgePassage records bounded retrieval evidence supplied to inference.
// It is not a claim that every interpretation in an answer is supported.
type KnowledgePassage struct {
	DocumentID  string `json:"document_id"`
	Name        string `json:"name"`
	ChunkID     string `json:"chunk_id"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}
