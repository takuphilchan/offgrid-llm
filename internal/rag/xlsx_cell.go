package rag

import (
	"fmt"
	"strconv"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func xlsxTypedCell(c xlsxCell, shared []xlsxString, formatID int, format string, options xlsxOptions, anchors map[int]xlsxCell) (*document.Cell, error) {
	value := ""
	if c.Value != nil {
		value = *c.Value
	}
	cell := &document.Cell{Reference: c.Ref, Value: value, Display: value, DisplayState: "stored", NumberFormat: format, NumberFormatID: formatID, ResultState: "literal"}
	switch c.Type {
	case "inlineStr":
		cell.ValueType = "text"
		cell.Value = c.Inline.value()
		cell.Display = cell.Value
	case "s":
		i, err := strconv.Atoi(value)
		if err != nil || i < 0 || i >= len(shared) {
			return nil, fmt.Errorf("invalid shared string reference")
		}
		cell.ValueType = "text"
		cell.Value = shared[i].value()
		cell.Display = cell.Value
	case "str":
		cell.ValueType = "text"
	case "b":
		cell.ValueType = "boolean"
		if value == "1" {
			cell.Display = "TRUE"
		} else if value == "0" {
			cell.Display = "FALSE"
		} else if c.Formula == nil || value != "" {
			return nil, fmt.Errorf("invalid boolean cell")
		}
	case "e":
		cell.ValueType = "error"
	case "d":
		cell.ValueType = "date"
		valid := false
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02"} {
			if _, err := time.Parse(layout, value); err == nil {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid ISO date cell")
		}
	case "", "n":
		cell.ValueType = "number"
		if value != "" {
			if _, ok := xlsxRational(value); !ok {
				return nil, fmt.Errorf("invalid numeric cell")
			}
			cell.Display, cell.DisplayState = xlsxDisplayNumber(value, format, options.date1904)
		}
		cell.DateSystem = "1900"
		if options.date1904 {
			cell.DateSystem = "1904"
		}
	default:
		return nil, fmt.Errorf("unsupported cell value type")
	}
	if c.Formula != nil {
		f := c.Formula
		kind := f.Type
		if kind == "" {
			kind = "normal"
		}
		switch kind {
		case "normal", "shared", "array", "dataTable":
		default:
			return nil, fmt.Errorf("invalid formula type")
		}
		if f.Shared != nil && *f.Shared < 0 {
			return nil, fmt.Errorf("invalid shared formula index")
		}
		cell.Formula = f.Text
		cell.FormulaType = kind
		cell.SharedFormulaIndex = f.Shared
		cell.FormulaReference = f.Reference
		if f.Reference != "" {
			if _, ok := xlsxRange(f.Reference); !ok {
				return nil, fmt.Errorf("invalid formula range")
			}
		}
		if kind == "shared" && f.Shared != nil && f.Text == "" {
			if anchor, ok := anchors[*f.Shared]; ok {
				cell.FormulaAnchor = anchor.Ref
				if area, ok := xlsxRange(anchor.Formula.Reference); ok && !area.contains(c.Ref) {
					return nil, fmt.Errorf("shared formula outside anchor range")
				}
			}
		}
		cell.CachedValue = c.Value
		if c.Value != nil && *c.Value == "" && cell.ValueType != "text" {
			cell.CachedValue = nil
		}
		cell.ResultState = "cached_unverified"
		if options.manual {
			cell.ResultState = "cached_manual_calculation"
		}
		if options.recalculate {
			cell.ResultState = "cached_recalculation_requested"
		}
		if cell.ValueType == "error" {
			cell.ResultState = "cached_error_unverified"
		}
		if cell.CachedValue == nil {
			cell.ResultState = "missing_cache"
		}
	}
	return cell, nil
}
