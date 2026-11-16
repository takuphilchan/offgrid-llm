package rag

import (
	"fmt"
	"sort"
)

// Map cells to disjoint rectangles with a row sweep and a bounded column tree.
// Work is O((ranges + cells) log 16384), including empty ranges. Expanding large
// rectangles or scanning every range per cell would allow hostile workbooks to
// consume unbounded CPU despite the package's byte limits.
func xlsxAreaOwners(areas []xlsxArea, references []string) ([]int, error) {
	type event struct {
		row, owner int
		remove     bool
	}
	events := make([]event, 0, len(areas)*2)
	for i, a := range areas {
		events = append(events, event{a.firstRow, i, false}, event{a.lastRow + 1, i, true})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].row != events[j].row {
			return events[i].row < events[j].row
		}
		return events[i].remove && !events[j].remove
	})
	type point struct{ row, col, index int }
	points := make([]point, len(references))
	owners := make([]int, len(references))
	for i, ref := range references {
		col, row := xlsxCellPosition(ref)
		points[i] = point{row, col, i}
		owners[i] = -1
	}
	sort.Slice(points, func(i, j int) bool { return points[i].row < points[j].row })
	tree := xlsxColumnTree{maximum: make([]int, 4*16384), lazy: make([]int, 4*16384)}
	e := 0
	advance := func(row int) error {
		for e < len(events) && events[e].row <= row {
			event := events[e]
			a := areas[event.owner]
			owner := 0
			if !event.remove {
				if tree.query(1, 1, 16384, a.firstCol, a.lastCol) != 0 {
					return fmt.Errorf("overlapping worksheet ranges")
				}
				owner = event.owner + 1
			}
			tree.assign(1, 1, 16384, a.firstCol, a.lastCol, owner)
			e++
		}
		return nil
	}
	for _, p := range points {
		if err := advance(p.row); err != nil {
			return nil, err
		}
		owners[p.index] = tree.query(1, 1, 16384, p.col, p.col) - 1
	}
	if err := advance(1048577); err != nil {
		return nil, err
	}
	return owners, nil
}

type xlsxColumnTree struct{ maximum, lazy []int }

func (t *xlsxColumnTree) push(n int) {
	if t.lazy[n] < 0 {
		return
	}
	for _, child := range []int{n * 2, n*2 + 1} {
		t.maximum[child], t.lazy[child] = t.lazy[n], t.lazy[n]
	}
	t.lazy[n] = -1
}
func (t *xlsxColumnTree) assign(n, left, right, first, last, owner int) {
	if first <= left && right <= last {
		t.maximum[n], t.lazy[n] = owner, owner
		return
	}
	t.push(n)
	mid := (left + right) / 2
	if first <= mid {
		t.assign(n*2, left, mid, first, last, owner)
	}
	if last > mid {
		t.assign(n*2+1, mid+1, right, first, last, owner)
	}
	t.maximum[n] = max(t.maximum[n*2], t.maximum[n*2+1])
}
func (t *xlsxColumnTree) query(n, left, right, first, last int) int {
	if first <= left && right <= last {
		return t.maximum[n]
	}
	if t.lazy[n] >= 0 {
		return t.lazy[n]
	}
	mid, value := (left+right)/2, 0
	if first <= mid {
		value = t.query(n*2, left, mid, first, last)
	}
	if last > mid {
		value = max(value, t.query(n*2+1, mid+1, right, first, last))
	}
	return value
}
