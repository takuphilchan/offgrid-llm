package rag

import (
	"fmt"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// Typed metadata is retained in the graph, not duplicated into every retrieval
// locator. Header associations come only from explicit workbook tables.
func (r *ParseResult) enrichXLSXStructure() error {
	if r.Metadata["format"] != "xlsx" {
		return nil
	}
	graph := r.Structure
	positions := map[string]int{}
	for i, b := range graph.Blocks {
		positions[b.ID] = i
	}
	for _, unit := range r.SourceUnits {
		unitID := fmt.Sprintf("sheet-%d", unit.Index)
		for i, u := range graph.Units {
			if u.ID == unitID {
				graph.Units[i].Hidden = unit.Hidden
				graph.Units[i].MergedRanges = append([]string(nil), unit.MergedRanges...)
			}
		}
		rows := map[int]document.Span{}
		for _, cell := range unit.Cells {
			id := unitID + "-cell-" + cell.Reference
			at, ok := positions[id]
			if !ok {
				return fmt.Errorf("typed workbook cell has no exact block")
			}
			if cell.Data != nil {
				data := *cell.Data
				graph.Blocks[at].Cell = &data
			}
			_, row := xlsxCellPosition(cell.Reference)
			span, found := rows[row]
			if !found {
				span = graph.Blocks[at].Span
			} else {
				span.End = graph.Blocks[at].Span.End
			}
			rows[row] = span
			for _, header := range cell.Headers {
				headerID := unitID + "-cell-" + header
				if _, ok := positions[headerID]; !ok {
					return fmt.Errorf("table header block missing")
				}
				graph.Relationships = append(graph.Relationships, document.Relationship{From: headerID, To: id, Kind: "header_for"})
			}
			graph.Relationships = append(graph.Relationships, document.Relationship{From: id, To: fmt.Sprintf("%s-row-%d", unitID, row), Kind: "parent"})
		}
		order := make([]int, 0, len(rows))
		for row := range rows {
			order = append(order, row)
		}
		sort.Ints(order)
		for _, row := range order {
			graph.Blocks = append(graph.Blocks, document.Block{ID: fmt.Sprintf("%s-row-%d", unitID, row), UnitID: unitID, Kind: "row", Span: rows[row], Method: "native"})
		}
	}
	return nil
}
