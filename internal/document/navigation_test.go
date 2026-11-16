package document

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNavigationPinsExactUnitOffsetsAndBoundsUnicode(t *testing.T) {
	e := sampleExtraction()
	e.Text = strings.Repeat("政策", 100)
	e.Units = []Unit{{ID: "first", Kind: "section", Index: 1, Span: Span{End: len(e.Text)}, State: "ready"}, {ID: "second", Kind: "section", Index: 2, Span: Span{End: len(e.Text)}, State: "partial"}}
	e.Blocks = nil
	e.Relationships = nil
	for i := 0; i < 207; i++ {
		unit := "first"
		if i >= 103 {
			unit = "second"
		}
		kind := "heading"
		if i == 104 {
			kind = "figure"
		}
		b := Block{ID: fmt.Sprint("block-", i), UnitID: unit, Kind: kind, Method: "native", Span: Span{End: len(e.Text)}}
		if i == 105 {
			b.Warnings = []Warning{{Code: "uncertain", Message: "Requires source review"}}
		}
		e.Blocks = append(e.Blocks, b)
	}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	first, err := ProjectStructure(e, "doc", "", 0, 1, 0, false)
	if err != nil || len(first.Navigation) != 100 || first.TotalNavigation != 207 || first.NextNavigationOffset == nil || *first.NextNavigationOffset != 100 {
		t.Fatal(first, err)
	}
	if first.Units[1].BlockCount != 104 || first.Units[1].FigureBlocks != 1 || first.Units[1].ReviewBlocks != 1 {
		t.Fatal(first.Units)
	}
	for _, entry := range first.Navigation {
		if len(entry.Title) > 256 || !utf8.ValidString(entry.Title) {
			t.Fatal("broken title", entry)
		}
	}
	next, err := ProjectStructure(e, "doc", "", 0, 1, 0, false, 100)
	if err != nil || next.Navigation[3].UnitID != "second" || next.Navigation[3].BlockOffset != 0 {
		t.Fatal(next, err)
	}
	target := next.Navigation[5]
	jumped, err := ProjectStructure(e, "doc", target.UnitID, target.BlockOffset, 1, 0, false)
	if err != nil || len(jumped.Blocks) != 1 || jumped.Blocks[0].ID != target.BlockID || !target.NeedsReview {
		t.Fatal(jumped, target, err)
	}
	filtered, err := ProjectStructure(e, "doc", "second", 0, 1, 0, false, 100)
	if err != nil || filtered.TotalNavigation != 104 || len(filtered.Navigation) != 4 || filtered.NextNavigationOffset != nil {
		t.Fatal(filtered, err)
	}
	if e.Verify() != nil {
		t.Fatal("navigation mutated immutable source")
	}
	for _, offset := range []int{-1, 200001} {
		if _, err := ProjectStructure(e, "doc", "", 0, 1, 0, false, offset); err == nil {
			t.Fatal("accepted navigation offset", offset)
		}
	}
}
