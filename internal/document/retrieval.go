package document

// EvidenceReceipt describes the retained extraction actually used for an
// excerpt. Complete applies only to its explicit structural links, never to
// extraction quality or an exhaustive review of the document.
type EvidenceReceipt struct {
	Revision       string   `json:"extraction_revision"`
	BlockIDs       []string `json:"block_ids,omitempty"`
	GroupID        string   `json:"group_id"`
	RequiredChunks []string `json:"required_chunks,omitempty"`
	OmittedBlocks  []string `json:"omitted_blocks,omitempty"`
	LinksComplete  bool     `json:"links_complete"`
	LimitReached   bool     `json:"limit_reached,omitempty"`
}
