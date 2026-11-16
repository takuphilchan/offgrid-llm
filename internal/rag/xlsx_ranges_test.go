package rag

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestXLSXRangeSweepMatchesIndependentRectangleChecks(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	refs := []string{"A1", "D9", "J10", "B4", "XFD1048576"}
	for run := 0; run < 400; run++ {
		areas := make([]xlsxArea, random.Intn(12))
		for i := range areas {
			col, row := 1+random.Intn(10), 1+random.Intn(10)
			areas[i] = xlsxArea{col, row, col + random.Intn(4), row + random.Intn(4)}
		}
		overlap := false
		for i, a := range areas {
			for _, b := range areas[:i] {
				overlap = overlap || (a.firstCol <= b.lastCol && b.firstCol <= a.lastCol && a.firstRow <= b.lastRow && b.firstRow <= a.lastRow)
			}
		}
		owners, err := xlsxAreaOwners(areas, refs)
		if (err != nil) != overlap {
			t.Fatalf("overlap mismatch: %+v: %v", areas, err)
		}
		if overlap {
			continue
		}
		for i, ref := range refs {
			want := -1
			for j, a := range areas {
				if a.contains(ref) {
					want = j
				}
			}
			if owners[i] != want {
				t.Fatalf("%s: got %d want %d", ref, owners[i], want)
			}
		}
	}
}

func TestXLSXRangeSweepHandlesLargeSparseRangesAndEmptyOverlap(t *testing.T) {
	areas := make([]xlsxArea, 10000)
	refs := make([]string, 100000)
	for i := range areas {
		areas[i] = xlsxArea{i + 1, 1, i + 1, 1048576}
	}
	for i := range refs {
		refs[i] = fmt.Sprintf("A%d", i+1)
	}
	owners, err := xlsxAreaOwners(areas, refs)
	if err != nil || len(owners) != len(refs) {
		t.Fatal(err)
	}
	for _, owner := range owners {
		if owner != 0 {
			t.Fatal("lost tall range")
		}
	}
	if _, err = xlsxAreaOwners([]xlsxArea{{1, 100, 5, 200}, {3, 150, 6, 300}}, nil); err == nil {
		t.Fatal("empty overlapping ranges admitted")
	}
}
