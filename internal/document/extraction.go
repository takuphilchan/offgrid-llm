package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const ExtractionSchema = 1

// Span always addresses UTF-8 bytes in this revision's Text. Original source
// geometry is separate; a byte position is never presented as a page coordinate.
type Span struct {
	Start int `json:"start_byte"`
	End   int `json:"end_byte"`
}

type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"` // points, pixels or normalized; never implicit
}

type RuntimeIdentity struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
	SHA256   string `json:"sha256,omitempty"`
}

type Unit struct {
	Hidden         bool     `json:"hidden,omitempty"`
	MergedRanges   []string `json:"merged_ranges,omitempty"`
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Index          int      `json:"index"`
	Name           string   `json:"name,omitempty"`
	Span           Span     `json:"span"`
	State          string   `json:"state"`
	ParserRevision string   `json:"parser_revision"`
}

type Cell struct {
	RowSpan            int      `json:"row_span,omitempty"`
	ColumnSpan         int      `json:"column_span,omitempty"`
	HorizontalMerge    bool     `json:"horizontal_merge,omitempty"`
	VerticalMerge      bool     `json:"vertical_merge,omitempty"`
	DisplayState       string   `json:"display_state,omitempty"`
	NumberFormatID     int      `json:"number_format_id,omitempty"`
	DateSystem         string   `json:"date_system,omitempty"`
	FormulaType        string   `json:"formula_type,omitempty"`
	FormulaReference   string   `json:"formula_reference,omitempty"`
	SharedFormulaIndex *int     `json:"shared_formula_index,omitempty"`
	FormulaAnchor      string   `json:"formula_anchor,omitempty"`
	HiddenReasons      []string `json:"hidden_reasons,omitempty"`
	Reference          string   `json:"reference"`
	ValueType          string   `json:"value_type,omitempty"`
	Value              string   `json:"value,omitempty"`
	Display            string   `json:"display,omitempty"`
	NumberFormat       string   `json:"number_format,omitempty"`
	Formula            string   `json:"formula,omitempty"`
	CachedValue        *string  `json:"cached_value,omitempty"`
	ResultState        string   `json:"result_state,omitempty"`
	MergedRange        string   `json:"merged_range,omitempty"`
	Hidden             bool     `json:"hidden,omitempty"`
}

type Figure struct {
	Kind           string `json:"kind"`
	Part           string `json:"part,omitempty"`
	RelationshipID string `json:"relationship_id,omitempty"`
	External       bool   `json:"external,omitempty"`
	Name           string `json:"name,omitempty"`
	Title          string `json:"title,omitempty"`
	Description    string `json:"description,omitempty"`
}

type Table struct {
	Rows             int  `json:"rows"`
	Columns          int  `json:"columns"`
	FirstRowStyle    bool `json:"first_row_style,omitempty"`
	FirstColumnStyle bool `json:"first_column_style,omitempty"`
}

type FormOption struct {
	Index    int    `json:"index"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}

type FormField struct {
	Label        string       `json:"label,omitempty"`
	Options      []FormOption `json:"options,omitempty"`
	Name         string       `json:"name"`
	Value        string       `json:"value"`
	Kind         string       `json:"kind"`
	Checked      *bool        `json:"checked,omitempty"`
	Hidden       bool         `json:"hidden,omitempty"`
	NativeBounds *[4]float64  `json:"native_bounds,omitempty"` // PDF user-space left,bottom,right,top in points.
}

type RevisionChange struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Author string `json:"author,omitempty"`
	Date   string `json:"date,omitempty"`
	Text   string `json:"text"`
}

type Block struct {
	Revisions    []RevisionChange `json:"revisions,omitempty"`
	FormField    *FormField       `json:"form_field,omitempty"`
	Figure       *Figure          `json:"figure,omitempty"`
	Table        *Table           `json:"table,omitempty"`
	ID           string           `json:"id"`
	UnitID       string           `json:"unit_id"`
	Kind         string           `json:"kind"`
	Span         Span             `json:"span"`
	Method       string           `json:"method"`
	HeadingLevel int              `json:"heading_level,omitempty"`
	Cell         *Cell            `json:"cell,omitempty"`
	Bounds       *Bounds          `json:"bounds,omitempty"`
	Warnings     []Warning        `json:"warnings,omitempty"`
	ReviewState  string           `json:"review_state,omitempty"`
	SourcePart   string           `json:"source_part,omitempty"`
}

type Relationship struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Extraction is immutable after Seal. A correction creates a new extraction
// with ParentID and attributed provenance; it never rewrites an original file.
type Extraction struct {
	Schema         int               `json:"schema"`
	ID             string            `json:"id"`
	ParentID       string            `json:"parent_id,omitempty"`
	SourceKind     string            `json:"source_kind"`
	OriginalSHA256 string            `json:"original_sha256,omitempty"`
	Format         string            `json:"format"`
	Method         string            `json:"method"`
	Parsers        []RuntimeIdentity `json:"parsers"`
	LanguagePacks  []RuntimeIdentity `json:"language_packs,omitempty"`
	Languages      []string          `json:"languages,omitempty"`
	Coverage       *Coverage         `json:"coverage,omitempty"`
	Text           string            `json:"text"`
	Units          []Unit            `json:"units"`
	Blocks         []Block           `json:"blocks"`
	Relationships  []Relationship    `json:"relationships,omitempty"`
	Author         string            `json:"author,omitempty"`
	CreatedAt      *time.Time        `json:"created_at,omitempty"`
}

func (e *Extraction) fingerprint() (string, error) {
	copy := *e
	copy.ID = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return "extraction-" + hex.EncodeToString(hash[:]), nil
}

func (e *Extraction) Seal() error {
	if e == nil {
		return invalidExtraction("bounds")
	}
	e.Schema = ExtractionSchema
	if err := e.validate(); err != nil {
		return err
	}
	id, err := e.fingerprint()
	if err == nil {
		e.ID = id
	}
	return err
}

func (e *Extraction) Verify() error {
	if err := e.validate(); err != nil {
		return err
	}
	id, err := e.fingerprint()
	if err != nil {
		return err
	}
	if id != e.ID {
		return invalidExtraction("revision digest")
	}
	return nil
}
