package api

// KnowledgePassage records bounded retrieval evidence supplied to inference.
// It is not a claim that every interpretation in an answer is supported.
type KnowledgePassage struct {
	Source      *KnowledgeSourceLocator `json:"source,omitempty"`
	DocumentID  string                  `json:"document_id"`
	Name        string                  `json:"name"`
	ChunkID     string                  `json:"chunk_id"`
	Content     string                  `json:"content"`
	ContentHash string                  `json:"content_hash"`
}

// KnowledgeSourceLocator describes the actual excerpt, not a model-authored
// citation. Byte offsets refer to retained extracted text; unit identity comes
// from the parser that consumed the original source digest.
type KnowledgeSourceLocator struct {
	Cells     []string `json:"cells,omitempty"`
	Kind      string   `json:"kind"`
	Index     int      `json:"index"`
	Name      string   `json:"name,omitempty"`
	StartByte int      `json:"start_byte"`
	EndByte   int      `json:"end_byte"`
	SHA256    string   `json:"source_sha256"`
	Revision  string   `json:"extractor_revision"`
}
