package rag

import (
	"fmt"
	"strings"
)

type xlsxArea struct{ firstCol, firstRow, lastCol, lastRow int }

func xlsxRange(ref string) (xlsxArea, bool) {
	parts := strings.Split(ref, ":")
	if len(parts) < 1 || len(parts) > 2 {
		return xlsxArea{}, false
	}
	if !validCellReference(parts[0]) {
		return xlsxArea{}, false
	}
	end := parts[0]
	if len(parts) == 2 {
		end = parts[1]
	}
	if !validCellReference(end) {
		return xlsxArea{}, false
	}
	a, b := xlsxCellPosition(parts[0])
	c, d := xlsxCellPosition(end)
	return xlsxArea{a, b, c, d}, a <= c && b <= d
}
func (a xlsxArea) contains(ref string) bool {
	col, row := xlsxCellPosition(ref)
	return col >= a.firstCol && col <= a.lastCol && row >= a.firstRow && row <= a.lastRow
}

func bindXLSXTables(pkg *officePackage, part string, sheet *xlsxSheetResult) error {
	if len(sheet.tables) == 0 {
		return nil
	}
	relations, err := pkg.relations(part)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var areas []xlsxArea
	var hasHeaders []bool
	for _, id := range sheet.tables {
		if seen[id] {
			return fmt.Errorf("duplicate worksheet table")
		}
		seen[id] = true
		file, err := officePart(part, id, "table", relations)
		if err != nil {
			return err
		}
		var table struct {
			Ref     string `xml:"ref,attr"`
			Headers *int   `xml:"headerRowCount,attr"`
			Columns []struct {
				Name string `xml:"name,attr"`
			} `xml:"tableColumns>tableColumn"`
		}
		if err = pkg.decode(file, &table); err != nil {
			return err
		}
		area, ok := xlsxRange(table.Ref)
		if !ok {
			return fmt.Errorf("invalid worksheet table range")
		}
		headerRows := 1
		if table.Headers != nil {
			headerRows = *table.Headers
		}
		if headerRows < 0 || headerRows > 1 {
			return fmt.Errorf("invalid table header count")
		}
		if len(table.Columns) > 0 && len(table.Columns) != area.lastCol-area.firstCol+1 {
			return fmt.Errorf("table columns disagree with range")
		}
		areas = append(areas, area)
		hasHeaders = append(hasHeaders, headerRows == 1)
	}
	references := make([]string, len(sheet.cells))
	for i, cell := range sheet.cells {
		references[i] = cell.Reference
	}
	owners, err := xlsxAreaOwners(areas, references)
	if err != nil {
		return err
	}
	type headerKey struct{ table, column int }
	headers := map[headerKey]string{}
	for i, c := range sheet.cells {
		owner := owners[i]
		if owner < 0 || !hasHeaders[owner] {
			continue
		}
		col, row := xlsxCellPosition(c.Reference)
		if row == areas[owner].firstRow && c.Data != nil && strings.TrimSpace(c.Data.Value) != "" {
			headers[headerKey{owner, col}] = c.Reference
		}
	}
	for i, c := range sheet.cells {
		owner := owners[i]
		if owner < 0 || !hasHeaders[owner] {
			continue
		}
		col, row := xlsxCellPosition(c.Reference)
		if row <= areas[owner].firstRow {
			continue
		}
		if header := headers[headerKey{owner, col}]; header != "" {
			sheet.cells[i].Headers = []string{header}
		} else {
			sheet.warn("document_table_header_missing", "A declared table header has no retained cell value. Do not infer the missing label from column position.")
		}
	}
	return nil
}
