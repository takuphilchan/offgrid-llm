package rag

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func presentationSpan(n *presentationNode, name string) (int, error) {
	raw := n.attr(name)
	if raw == "" {
		return 1, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 10000 {
		return 0, fmt.Errorf("invalid presentation table span")
	}
	return value, nil
}
func presentationBool(raw string) bool { return raw == "1" || raw == "true" || raw == "on" }

func (b *presentationBuilder) table(n *presentationNode) error {
	grid := n.child("tblGrid")
	columns := 0
	if grid != nil {
		columns = len(grid.children)
	}
	var rows []*presentationNode
	for _, r := range n.children {
		if r.name.Local == "tr" && r.drawing() {
			rows = append(rows, r)
		}
	}
	if len(rows) > 10000 || columns > 10000 {
		return &ParseError{Code: "document_limit", Message: "Presentation table exceeds supported bounds."}
	}
	table, err := b.add("table", "")
	if err != nil {
		return err
	}
	meta := &document.Table{Rows: len(rows), Columns: columns}
	if properties := n.child("tblPr"); properties != nil {
		meta.FirstRowStyle = presentationBool(properties.attr("firstRow"))
		meta.FirstColumnStyle = presentationBool(properties.attr("firstCol"))
	}
	b.blocks[table].Table = meta
	// firstRow is a formatting flag, not a semantic header declaration. Retain
	// it for inspection without manufacturing header_for relationships.
	b.warn("document_table_header_review", "Presentation table styling does not establish semantic headers. Inspect the row and column labels before interpreting values.")
	for ri, row := range rows {
		r, err := b.add("row", "")
		if err != nil {
			return err
		}
		b.parent(r, table)
		var cells []*presentationNode
		for _, c := range row.children {
			if c.name.Local == "tc" && c.drawing() {
				cells = append(cells, c)
			}
		}
		if len(cells) > 10000 || (columns > 0 && len(cells) != columns) {
			return fmt.Errorf("presentation table grid disagrees with row")
		}
		for ci, cell := range cells {
			rowSpan, err := presentationSpan(cell, "rowSpan")
			if err != nil {
				return err
			}
			colSpan, err := presentationSpan(cell, "gridSpan")
			if err != nil {
				return err
			}
			if rowSpan > len(rows)-ri || colSpan > len(cells)-ci {
				return fmt.Errorf("presentation merged cell exceeds table")
			}
			value := ""
			if body := cell.child("txBody"); body != nil {
				value = presentationText(body)
			}
			reference := fmt.Sprintf("T%d:R%dC%d", table+1, ri+1, ci+1)
			typed := &document.Cell{Reference: reference, ValueType: "text", Value: value, Display: value, DisplayState: "stored", ResultState: "literal", RowSpan: rowSpan, ColumnSpan: colSpan, HorizontalMerge: presentationBool(cell.attr("hMerge")), VerticalMerge: presentationBool(cell.attr("vMerge"))}
			text := reference + ": " + value
			if rowSpan > 1 || colSpan > 1 || typed.HorizontalMerge || typed.VerticalMerge {
				text += fmt.Sprintf(" [merge: rows=%d columns=%d horizontal=%t vertical=%t]", rowSpan, colSpan, typed.HorizontalMerge, typed.VerticalMerge)
				b.warn("document_table_merge_review", "Presentation merge attributes are retained; merged-header associations require inspection.")
			}
			at, err := b.add("cell", strings.TrimRight(text, " "))
			if err != nil {
				return err
			}
			b.blocks[at].Cell = typed
			b.parent(at, r)
		}
		b.blocks[r].Span.End = b.out.Len()
	}
	b.blocks[table].Span.End = b.out.Len()
	return nil
}
