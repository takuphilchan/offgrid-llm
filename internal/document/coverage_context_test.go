package document

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCoverageContextSummarizesWithoutAlteringReceipt(t *testing.T) {
	c := &Coverage{Status: "partial", TotalUnits: 1000, ReadableUnits: 1000}
	for i := 1; i <= 1000; i++ {
		c.ReviewUnits = append(c.ReviewUnits, i)
		c.Warnings = append(c.Warnings, Warning{Code: "document_visuals_unread", Message: strings.Repeat("图", 700), SourceIndex: i})
	}
	data := c.ContextJSON()
	if !json.Valid(data) || len(data) > 8192 || !strings.Contains(string(data), `"warning_count":1000`) || !strings.Contains(string(data), `"warning_summary":true`) {
		t.Fatal("unbounded or misleading receipt")
	}
	if len(c.Warnings) != 1000 || len([]rune(c.Warnings[0].Message)) != 700 || len(c.ReviewUnits) != 1000 {
		t.Fatal("retained evidence changed")
	}
}
