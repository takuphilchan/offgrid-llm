package document

import "sort"

// LinkIndex indexes an already verified immutable extraction. It is shared by
// private attachments and Knowledge; it does not confer access to either store.
type LinkIndex struct {
	graph        *Extraction
	blocks       map[string]Block
	parents      map[string]string
	dependencies map[string][]string
	intervals    *blockIntervals
}

type LinkedBlocks struct {
	BlockIDs     []string
	Required     []Block
	LimitReached bool
}

func IndexLinks(graph *Extraction) *LinkIndex {
	index := &LinkIndex{graph: graph, blocks: map[string]Block{}, parents: map[string]string{}, dependencies: map[string][]string{}}
	for _, b := range graph.Blocks {
		index.blocks[b.ID] = b
	}
	index.intervals = newBlockIntervals(graph.Blocks)
	for _, r := range graph.Relationships {
		switch r.Kind {
		case "parent":
			index.parents[r.From] = r.To
		case "heading_for", "header_for", "footnote_for", "caption_for", "notes_for":
			index.dependencies[r.To] = append(index.dependencies[r.To], r.From)
			if r.Kind == "footnote_for" {
				index.dependencies[r.From] = append(index.dependencies[r.From], r.To)
			}
		}
	}
	return index
}

// ForSpan follows retained structural dependencies, not inferred meaning. A
// value can find its row's header, and a note can find its governing clause.
// Required spans remain exact source text and are sorted outer-before-inner.
func (index *LinkIndex) ForSpan(span Span) LinkedBlocks {
	result := LinkedBlocks{}
	queue, required := []string{}, map[string]bool{}
	for _, ordinal := range index.intervals.overlaps(span) {
		b := index.graph.Blocks[ordinal]
		if b.Kind != "table" && b.Span.Start < span.End && b.Span.End > span.Start {
			queue = append(queue, b.ID)
			if len(result.BlockIDs) < 128 {
				result.BlockIDs = append(result.BlockIDs, b.ID)
			} else {
				result.LimitReached = true
			}
		}
	}
	visited := map[string]bool{}
	for at := 0; at < len(queue); at++ {
		id := queue[at]
		if visited[id] {
			continue
		}
		visited[id] = true
		if parent := index.parents[id]; parent != "" {
			queue = append(queue, parent)
		}
		for _, dependency := range index.dependencies[id] {
			required[dependency] = true
			queue = append(queue, dependency)
		}
	}
	for id := range required {
		result.Required = append(result.Required, index.blocks[id])
	}
	sort.Slice(result.Required, func(i, j int) bool {
		a, b := result.Required[i].Span, result.Required[j].Span
		if a.Start == b.Start {
			if a.End == b.End {
				return result.Required[i].ID < result.Required[j].ID
			}
			return a.End > b.End
		}
		return a.Start < b.Start
	})
	return result
}

// SelectionReceipt accounts for explicit links in the final attachment context.
// It is separate from extraction coverage and never proves an exhaustive review.
type SelectionReceipt struct {
	Revision      string   `json:"extraction_revision"`
	LinksComplete bool     `json:"links_complete"`
	RequiredSpans []Span   `json:"required_spans,omitempty"`
	OmittedBlocks []string `json:"omitted_blocks,omitempty"`
	LimitReached  bool     `json:"limit_reached,omitempty"`
}
