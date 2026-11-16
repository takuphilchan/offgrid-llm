package textsearch

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLiteralMultilingualTerms(t *testing.T) {
	for _, row := range []struct{ input, term string }{
		{"Case 42 / UK", "42"}, {"Case 42 / UK", "uk"},
		{"青岚项目预算是多少？", "预算"}, {"日本語の予算", "予算"},
		{"مرحبا بالعالم", "مرحبا"}, {"Cafe\u0301", "café"},
	} {
		if Frequencies(row.input)[row.term] == 0 {
			t.Errorf("%q missing %q", row.input, row.term)
		}
	}
	if q := FTSQuery(`harvest? "allocation" OR NOT (42*)`); strings.Contains(q, "?") || strings.Contains(q, "*") || !strings.Contains(q, `"42"`) {
		t.Fatal(q)
	}
	if got := len(QueryTerms(strings.Repeat("项目预算未知内容 ", 1000))); got > MaxQueryTerms {
		t.Fatal(got)
	}
}

func TestQueryBudgetDoesNotTruncateSourceProjection(t *testing.T) {
	input := strings.Repeat("字", MaxQueryBytes) + " finalidentifier"
	if got := boundedQuery(input); len(got) > MaxQueryBytes || !utf8.ValidString(got) {
		t.Fatal("invalid bounded query")
	}
	if strings.Contains(FTSQuery(input), "finalidentifier") {
		t.Fatal("query budget not enforced")
	}
	if Frequencies(input)["finalidentifier"] != 1 || !strings.Contains(IndexText(input), "finalidentifier") {
		t.Fatal("source projection truncated")
	}
}
