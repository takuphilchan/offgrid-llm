package document

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestBlockIntervalsMatchOriginalScanWithOverlappingContainers(t *testing.T) {
	random := rand.New(rand.NewSource(47))
	blocks := []Block{{Span: Span{0, 100000}}, {Span: Span{500, 500}}}
	for i := 0; i < 1000; i++ {
		start := random.Intn(100000)
		blocks = append(blocks, Block{Span: Span{start, start + random.Intn(1000)}})
	}
	index := newBlockIntervals(blocks)
	for i := 0; i < 1000; i++ {
		start := random.Intn(100000)
		span := Span{start, start + random.Intn(1000)}
		var expected []int
		for ordinal, b := range blocks {
			if b.Span.Start < span.End && b.Span.End > span.Start {
				expected = append(expected, ordinal)
			}
		}
		if actual := index.overlaps(span); !reflect.DeepEqual(actual, expected) {
			t.Fatal(span, actual, expected)
		}
	}
	if len(newBlockIntervals(nil).overlaps(Span{0, 10})) != 0 {
		t.Fatal("empty index")
	}
}

func TestLinkedDependencyOrderStableForIdenticalSpans(t *testing.T) {
	g := analysisFixture(t, "Heading\nClause")
	g.Blocks = []Block{{ID: "primary", UnitID: "unit", Kind: "paragraph", Method: "native", Span: Span{8, 14}}, {ID: "z", UnitID: "unit", Kind: "heading", Method: "native", Span: Span{0, 7}}, {ID: "a", UnitID: "unit", Kind: "heading", Method: "native", Span: Span{0, 7}}}
	g.Relationships = []Relationship{{From: "z", To: "primary", Kind: "heading_for"}, {From: "a", To: "primary", Kind: "heading_for"}}
	if err := g.Seal(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		linked := IndexLinks(g).ForSpan(Span{8, 14})
		if len(linked.Required) != 2 || linked.Required[0].ID != "a" || linked.Required[1].ID != "z" {
			t.Fatal(linked)
		}
	}
}

func BenchmarkBlockIntervalsSparseQuery(b *testing.B) {
	for _, count := range []int{20000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			blocks := make([]Block, count)
			for i := range blocks {
				blocks[i].Span = Span{i * 100, i*100 + 99}
			}
			index := newBlockIntervals(blocks)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if len(index.overlaps(Span{(count / 2) * 100, (count/2)*100 + 20})) != 1 {
					b.Fatal("missing block")
				}
			}
		})
	}
}
