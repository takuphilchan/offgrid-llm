package document

import (
	"encoding/json"
	"errors"
	"unicode/utf8"
)

var ErrStructureQuery = errors.New("invalid structured source query")

// ProjectStructure projects an already verified graph. Callers retain ownership,
// revision and access checks; this shared renderer never grants correction rights.
func ProjectStructure(graph *Extraction, id, unit string, offset, limit, relationshipOffset int, canCorrect bool, navigationOffset ...int) (*StructurePage, error) {
	navigation := 0
	if len(navigationOffset) > 1 {
		return nil, ErrStructureQuery
	}
	if len(navigationOffset) == 1 {
		navigation = navigationOffset[0]
	}
	if navigation < 0 || navigation > 200000 {
		return nil, ErrStructureQuery
	}
	if graph == nil || offset < 0 || offset > 200000 || limit < 1 || limit > 200 || len(unit) > 128 || relationshipOffset < 0 || relationshipOffset > 400000 {
		return nil, ErrStructureQuery
	}
	page := &StructurePage{DocumentID: id, Revision: graph.ID, ParentRevision: graph.ParentID, OriginalSHA256: graph.OriginalSHA256, Format: graph.Format, Method: graph.Method, Author: graph.Author, CreatedAt: graph.CreatedAt, Coverage: graph.Coverage, Units: []UnitView{}, Blocks: []BlockView{}, Relationships: []Relationship{}, Offset: offset, CanCorrect: canCorrect}
	var parsersOmitted, packsOmitted bool
	page.Parsers, parsersOmitted = projectRuntimeIdentities(graph.Parsers)
	page.LanguagePacks, packsOmitted = projectRuntimeIdentities(graph.LanguagePacks)
	page.ProvenanceOmitted = parsersOmitted || packsOmitted
	unitMetadataBudget := 128 << 10
	for _, u := range graph.Units {
		view := UnitView{Unit: u}
		encoded, _ := json.Marshal(u.MergedRanges)
		if len(encoded) > min(8192, unitMetadataBudget) {
			view.MergedRanges = nil
			view.MetadataOmitted = true
		} else {
			unitMetadataBudget -= len(encoded)
		}
		page.Units = append(page.Units, view)
	}
	if unit != "" {
		found := false
		for _, u := range graph.Units {
			found = found || u.ID == unit
		}
		if !found {
			return nil, ErrStructureQuery
		}
	}
	children := map[string]bool{}
	for _, r := range graph.Relationships {
		if r.Kind == "parent" || r.Kind == "footnote_for" {
			children[r.To] = true
		}
	}
	selected := map[string]bool{}
	budget := 256 << 10
	metadataBudget := 256 << 10
	for _, block := range graph.Blocks {
		if unit != "" && block.UnitID != unit {
			continue
		}
		ordinal := page.TotalBlocks
		page.TotalBlocks++
		if ordinal < offset || len(page.Blocks) >= limit {
			continue
		}
		end := min(block.Span.End, block.Span.Start+min(32768, budget))
		for end > block.Span.Start && end < len(graph.Text) && !utf8.RuneStart(graph.Text[end]) {
			end--
		}
		text := graph.Text[block.Span.Start:end]
		budget -= len(text)
		truncated := end < block.Span.End
		editable := canCorrect && !children[block.ID] && !truncated && block.Span.End > block.Span.Start
		switch block.Kind {
		case "paragraph", "heading", "list", "cell", "footnote", "endnote", "form_field", "caption", "header", "footer":
		default:
			editable = false
		}
		metadata, _ := json.Marshal(block)
		omitted := len(metadata) > min(8192, metadataBudget)
		if omitted {
			block.Revisions = nil
			block.FormField = nil
			block.Cell = nil
			block.Figure = nil
			block.Table = nil
			block.Warnings = nil
			editable = false
		} else {
			metadataBudget -= len(metadata)
		}
		page.Blocks = append(page.Blocks, BlockView{Block: block, Text: text, TextSHA256: TextSHA256(graph.Text[block.Span.Start:block.Span.End]), Truncated: truncated, Editable: editable, MetadataOmitted: omitted})
		selected[block.ID] = true
	}
	for _, r := range graph.Relationships {
		if selected[r.From] || selected[r.To] {
			if page.TotalRelationships >= relationshipOffset && len(page.Relationships) < 2000 {
				page.Relationships = append(page.Relationships, r)
			}
			page.TotalRelationships++
		}
	}
	page.RelationshipOffset = relationshipOffset
	if next := relationshipOffset + len(page.Relationships); next < page.TotalRelationships {
		page.NextRelationshipOffset = &next
	}
	if next := offset + len(page.Blocks); next < page.TotalBlocks {
		page.NextOffset = &next
	}
	projectNavigation(graph, page, unit, navigation)

	return page, nil
}

func projectRuntimeIdentities(items []RuntimeIdentity) ([]RuntimeIdentity, bool) {
	result := []RuntimeIdentity{}
	omitted := len(items) > 32
	for i, item := range items {
		if i >= 32 {
			break
		}
		if len(item.Name) > 256 || len(item.Revision) > 256 || len(item.SHA256) > 64 {
			omitted = true
			continue
		}
		result = append(result, item)
	}
	return result, omitted
}
