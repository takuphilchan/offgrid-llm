package document

import "unicode/utf8"

// Navigation uses verified retained structure, never a generated outline. The
// page is bounded independently of text and relationship pages. Full totals
// come from the immutable graph, not whichever excerpts happen to be loaded.
func projectNavigation(graph *Extraction, page *StructurePage, unit string, offset int) {
	page.Navigation = []NavigationEntry{}
	page.NavigationOffset = offset
	units := make(map[string]int, len(page.Units))
	for i, u := range page.Units {
		units[u.ID] = i
	}
	for _, b := range graph.Blocks {
		u := &page.Units[units[b.UnitID]]
		ordinal := u.BlockCount
		u.BlockCount++
		review := len(b.Warnings) > 0 || b.ReviewState != "" && b.ReviewState != "corrected" && b.ReviewState != "reviewed"
		if review {
			u.ReviewBlocks++
		}
		if b.Kind == "figure" {
			u.FigureBlocks++
		}
		if unit != "" && unit != b.UnitID || b.Kind != "heading" && b.Kind != "table" && b.Kind != "figure" && !review {
			continue
		}
		n := page.TotalNavigation
		page.TotalNavigation++
		if n < offset || len(page.Navigation) >= 100 {
			continue
		}
		end := min(b.Span.End, b.Span.Start+256)
		for end > b.Span.Start && end < len(graph.Text) && !utf8.RuneStart(graph.Text[end]) {
			end--
		}
		title := graph.Text[b.Span.Start:end]
		page.Navigation = append(page.Navigation, NavigationEntry{BlockID: b.ID, UnitID: b.UnitID, BlockOffset: ordinal, Kind: b.Kind, Title: title, HeadingLevel: b.HeadingLevel, NeedsReview: review})
	}
	if next := offset + len(page.Navigation); next < page.TotalNavigation {
		page.NextNavigationOffset = &next
	}
}
