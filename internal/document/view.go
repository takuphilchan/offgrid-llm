package document

import "time"

// StructurePage is a bounded reader projection, never the entire extraction
// graph/text. Revision pins pagination across concurrent edits.
type StructurePage struct {
	AttachmentRevision     string            `json:"attachment_revision,omitempty"`
	Navigation             []NavigationEntry `json:"navigation"`
	NavigationOffset       int               `json:"navigation_offset"`
	NextNavigationOffset   *int              `json:"next_navigation_offset"`
	TotalNavigation        int               `json:"total_navigation"`
	DocumentID             string            `json:"document_id"`
	Revision               string            `json:"revision"`
	ParentRevision         string            `json:"parent_revision,omitempty"`
	OriginalSHA256         string            `json:"original_sha256,omitempty"`
	Format                 string            `json:"format"`
	Method                 string            `json:"method"`
	Parsers                []RuntimeIdentity `json:"parsers,omitempty"`
	LanguagePacks          []RuntimeIdentity `json:"language_packs,omitempty"`
	ProvenanceOmitted      bool              `json:"provenance_omitted,omitempty"`
	Author                 string            `json:"author,omitempty"`
	CreatedAt              *time.Time        `json:"created_at,omitempty"`
	Coverage               *Coverage         `json:"coverage,omitempty"`
	Units                  []UnitView        `json:"units"`
	Blocks                 []BlockView       `json:"blocks"`
	Relationships          []Relationship    `json:"relationships"`
	RelationshipOffset     int               `json:"relationship_offset"`
	NextRelationshipOffset *int              `json:"next_relationship_offset"`
	TotalRelationships     int               `json:"total_relationships"`
	TotalBlocks            int               `json:"total_blocks"`
	Offset                 int               `json:"offset"`
	NextOffset             *int              `json:"next_offset"`
	CanCorrect             bool              `json:"can_correct"`
}

type UnitView struct {
	Unit
	BlockCount      int  `json:"block_count"`
	ReviewBlocks    int  `json:"review_blocks"`
	FigureBlocks    int  `json:"figure_blocks"`
	MetadataOmitted bool `json:"metadata_omitted,omitempty"`
}

// BlockOffset addresses the block inside UnitID, independent of navigation
// pagination. Entries never replace the complete retained source text.
type NavigationEntry struct {
	BlockID      string `json:"block_id"`
	UnitID       string `json:"unit_id"`
	BlockOffset  int    `json:"block_offset"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	HeadingLevel int    `json:"heading_level,omitempty"`
	NeedsReview  bool   `json:"needs_review"`
}

type BlockView struct {
	Block
	Text            string `json:"text"`
	TextSHA256      string `json:"text_sha256"`
	Truncated       bool   `json:"truncated"`
	Editable        bool   `json:"editable"`
	MetadataOmitted bool   `json:"metadata_omitted"`
}

type CorrectionRequest struct {
	DocumentID       string       `json:"document_id"`
	ExpectedRevision string       `json:"expected_revision"`
	Edits            []Correction `json:"edits"`
}
