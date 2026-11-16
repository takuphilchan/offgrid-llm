package document

import "sort"

// An augmented start-ordered index prunes unrelated blocks without losing long
// overlapping containers. Returned ordinals preserve the original graph order.
type blockIntervals struct {
	blocks []Block
	order  []int
	ends   []int
}

func newBlockIntervals(blocks []Block) *blockIntervals {
	x := &blockIntervals{blocks: blocks, order: make([]int, len(blocks)), ends: make([]int, 4*len(blocks)+1)}
	for i := range blocks {
		x.order[i] = i
	}
	sort.SliceStable(x.order, func(i, j int) bool { return blocks[x.order[i]].Span.Start < blocks[x.order[j]].Span.Start })
	if len(blocks) > 0 {
		x.build(1, 0, len(blocks))
	}
	return x
}
func (x *blockIntervals) build(node, lo, hi int) int {
	if hi-lo == 1 {
		x.ends[node] = x.blocks[x.order[lo]].Span.End
	} else {
		mid := (lo + hi) / 2
		x.ends[node] = max(x.build(node*2, lo, mid), x.build(node*2+1, mid, hi))
	}
	return x.ends[node]
}
func (x *blockIntervals) overlaps(span Span) []int {
	var result []int
	var visit func(int, int, int)
	visit = func(node, lo, hi int) {
		if lo >= hi || x.ends[node] <= span.Start || x.blocks[x.order[lo]].Span.Start >= span.End {
			return
		}
		if hi-lo == 1 {
			result = append(result, x.order[lo])
			return
		}
		mid := (lo + hi) / 2
		visit(node*2, lo, mid)
		visit(node*2+1, mid, hi)
	}
	if len(x.order) > 0 {
		visit(1, 0, len(x.order))
	}
	sort.Ints(result)
	return result
}
