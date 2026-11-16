package rag

import (
	"encoding/csv"
	"encoding/json"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// CSV has a row per candidate, with conditions and exceptions beside its claim.
// Metadata rows preserve coverage, model and immutable source identities. Exact
// values remain in metadata_json if spreadsheet formula protection changes display.
type synthesisExportWriter struct{ *analysisExportWriter }

func newSynthesisExportWriter(format string) *synthesisExportWriter {
	return &synthesisExportWriter{newAnalysisExportWriter(format)}
}

func (w *synthesisExportWriter) record(kind string, value any) error {
	if w.format == "json" {
		return w.analysisExportWriter.record(kind, value)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var row struct {
		Batch       int                         `json:"batch"`
		Candidate   int                         `json:"candidate"`
		Value       document.SynthesisCandidate `json:"value"`
		HumanReview SynthesisDecision           `json:"human_review"`
	}
	if kind == "candidate" && json.Unmarshal(raw, &row) != nil {
		return ErrAnalysisInvalid
	}
	if w.format == "markdown" {
		if kind != "candidate" {
			return w.analysisExportWriter.record(kind, value)
		}
		_, err = w.Write([]byte("## Draft candidate · review required\n\n" + exportFence(row.Value.Text, "") + "### Evidence, conditions and exceptions\n\n" + exportFence(string(raw), "json")))
		w.records++
		return err
	}
	c := csv.NewWriter(w)
	if w.records == 0 {
		if err = c.Write([]string{"record_type", "batch", "candidate", "draft_text", "evidence", "conditions", "exceptions", "verification", "metadata_json", "human_decision", "review_note"}); err != nil {
			return err
		}
	}
	fields := make([]string, 11)
	fields[0], fields[8] = kind, string(raw)
	if kind == "candidate" {
		fields[9], fields[10] = row.HumanReview.Decision, row.HumanReview.Note
		batch, _ := json.Marshal(row.Batch)
		index, _ := json.Marshal(row.Candidate)
		fields[1], fields[2], fields[3], fields[7] = string(batch), string(index), row.Value.Text, row.Value.Verification
		for i, quotes := range [][]document.SynthesisCitedQuote{row.Value.Evidence, row.Value.Conditions, row.Value.Exceptions} {
			texts := make([]string, len(quotes))
			for j, q := range quotes {
				texts[j] = q.Text
			}
			fields[4+i] = strings.Join(texts, "\n\n")
		}
	}
	for i := range fields {
		fields[i] = spreadsheetText(fields[i])
	}
	err = c.Write(fields)
	c.Flush()
	if err == nil {
		err = c.Error()
	}
	w.records++
	return err
}
