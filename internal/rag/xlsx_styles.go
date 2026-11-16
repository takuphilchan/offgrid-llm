package rag

import "fmt"

type xlsxStyle struct {
	Format *int  `xml:"numFmtId,attr"`
	Base   *int  `xml:"xfId,attr"`
	Apply  *bool `xml:"applyNumberFormat,attr"`
}

type xlsxStyles struct {
	formats map[int]string
	cells   []int
}

func defaultXLSXStyles() xlsxStyles {
	// Locale-dependent date built-ins (including 14 and 22) have no unique
	// original display. Retain their ID instead of assuming US date order.
	return xlsxStyles{formats: map[int]string{0: "General", 1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00", 9: "0%", 10: "0.00%", 11: "0.00E+00", 12: "# ?/?", 13: "# ??/??", 20: "h:mm", 21: "h:mm:ss", 37: "#,##0_);(#,##0)", 38: "#,##0_);[Red](#,##0)", 39: "#,##0.00_);(#,##0.00)", 40: "#,##0.00_);[Red](#,##0.00)", 45: "mm:ss", 46: "[h]:mm:ss", 47: "mm:ss.0", 48: "##0.0E+0", 49: "@"}, cells: []int{0}}
}

func readXLSXStyles(pkg *officePackage, part string) (xlsxStyles, error) {
	styles := defaultXLSXStyles()
	var source struct {
		Formats []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Cells []xlsxStyle `xml:"cellXfs>xf"`
		Bases []xlsxStyle `xml:"cellStyleXfs>xf"`
	}
	if err := pkg.decode(part, &source); err != nil {
		return styles, err
	}
	if len(source.Formats) > 10000 || len(source.Cells) > 65536 || len(source.Bases) > 65536 {
		return styles, &ParseError{Code: "document_limit", Message: "Workbook styles exceed supported bounds."}
	}
	seen := map[int]bool{}
	for _, f := range source.Formats {
		if f.ID < 0 || f.ID > 65535 || seen[f.ID] || len(f.Code) > 2048 {
			return styles, fmt.Errorf("invalid workbook number format")
		}
		seen[f.ID] = true
		styles.formats[f.ID] = f.Code
	}
	if len(source.Cells) > 0 {
		styles.cells = nil
	}
	for _, xf := range source.Cells {
		id := 0
		if xf.Base != nil {
			if *xf.Base < 0 || *xf.Base >= len(source.Bases) {
				return styles, fmt.Errorf("invalid base cell style")
			}
			if base := source.Bases[*xf.Base].Format; base != nil {
				id = *base
			}
		}
		if xf.Format != nil && (xf.Apply == nil || *xf.Apply || xf.Base == nil) {
			id = *xf.Format
		}
		if id < 0 || id > 65535 {
			return styles, fmt.Errorf("invalid cell number format")
		}
		styles.cells = append(styles.cells, id)
	}
	return styles, nil
}

func (s xlsxStyles) format(style int) (int, string, error) {
	if style < 0 || style >= len(s.cells) {
		return 0, "", fmt.Errorf("invalid workbook style reference")
	}
	id := s.cells[style]
	if code, ok := s.formats[id]; ok {
		return id, code, nil
	}
	// Locale-specific built-ins and absent custom definitions are preserved,
	// never guessed as a currency or a related calendar.
	return id, fmt.Sprintf("unresolved-numFmtId:%d", id), nil
}
