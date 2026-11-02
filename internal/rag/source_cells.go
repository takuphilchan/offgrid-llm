package rag

import (
	"sort"
	"strings"
)

// SourceCell identifies an actual stored worksheet cell, including its displayed
// formula/cache annotation. Document ranges use absolute UTF-8 byte offsets;
// parser units use offsets relative to the sheet text.
type SourceCell struct {
	Reference string `json:"reference"`
	Start     int    `json:"start_byte"`
	End       int    `json:"end_byte"`
}

func validCellReference(ref string) bool {
	if len(ref) < 2 || len(ref) > 10 {
		return false
	}
	col, i := 0, 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A') + 1
		if col > 16384 {
			return false
		}
		i++
	}
	if col < 1 || col > 16384 || i == 0 || i == len(ref) || ref[i] == '0' {
		return false
	}
	row := 0
	for ; i < len(ref); i++ {
		if ref[i] < '0' || ref[i] > '9' {
			return false
		}
		row = row*10 + int(ref[i]-'0')
		if row > 1048576 {
			return false
		}
	}
	return row > 0
}

func sourceForSpan(source SourceRange, start, end int) *SourceRange {
	copy := source
	copy.Cells = nil
	first := sort.Search(len(source.Cells), func(i int) bool { return source.Cells[i].End > start })
	for _, cell := range source.Cells[first:] {
		if cell.Start >= end {
			break
		}
		copy.Cells = append(copy.Cells, cell)
	}
	return &copy
}

// CellReferences returns only cells intersecting the supplied excerpt, not all
// cells on the worksheet or cells removed by context truncation.
func (source SourceRange) CellReferences(start, end int) []string {
	var refs []string
	for _, cell := range sourceForSpan(source, start, end).Cells {
		refs = append(refs, cell.Reference)
	}
	return refs
}

func validateSourceCells(text string, source SourceRange) bool {
	if len(source.Cells) == 0 {
		return true
	}
	if source.Kind != "sheet" || len(source.Cells) > 100000 {
		return false
	}
	end := source.Start
	seen := map[string]bool{}
	for _, cell := range source.Cells {
		if !validCellReference(cell.Reference) || seen[cell.Reference] || cell.Start < end || cell.End <= cell.Start || cell.End > source.End || !strings.HasPrefix(text[cell.Start:cell.End], cell.Reference+":") {
			return false
		}
		seen[cell.Reference] = true
		end = cell.End
	}
	return true
}
