package rag

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"unicode"
)

type analysisExportWriter struct {
	format  string
	data    bytes.Buffer
	records int
}

func newAnalysisExportWriter(format string) *analysisExportWriter {
	return &analysisExportWriter{format: format}
}
func (w *analysisExportWriter) Write(p []byte) (int, error) {
	if w.data.Len()+len(p) > AnalysisExportLimit {
		return 0, ErrAnalysisLimit
	}
	return w.data.Write(p)
}
func (w *analysisExportWriter) record(kind string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	switch w.format {
	case "json":
		prefix := "[\n"
		if w.records > 0 {
			prefix = ",\n"
		}
		if _, err = w.Write([]byte(prefix)); err == nil {
			err = json.NewEncoder(w).Encode(struct {
				Type string `json:"type"`
				Data any    `json:"data"`
			}{kind, value})
		}
	case "csv":
		c := csv.NewWriter(w)
		if w.records == 0 {
			err = c.Write([]string{"record_type", "step", "change", "kind", "side", "document_id", "revision", "unit_id", "block_id", "start_byte", "end_byte", "text_sha256", "text", "metadata_json"})
		}
		if err != nil {
			return err
		}
		fields := make([]string, 14)
		fields[0] = kind
		fields[13] = string(raw)
		if kind == "change" {
			// JSON decoding preserves integer values and keeps the formatter independent
			// of storage. metadata_json retains the exact, unmodified evidence as well.
			var row map[string]json.RawMessage
			_ = json.Unmarshal(raw, &row)
			keys := []string{"step", "change", "kind", "side", "document_id"}
			for i, k := range keys {
				fields[i+1] = exportJSONValue(row[k])
			}
			var citation map[string]json.RawMessage
			_ = json.Unmarshal(row["citation"], &citation)
			for i, k := range []string{"revision", "unit_id", "block_id"} {
				fields[i+6] = exportJSONValue(citation[k])
			}
			var span map[string]json.RawMessage
			_ = json.Unmarshal(citation["span"], &span)
			fields[9] = string(span["start_byte"])
			fields[10] = string(span["end_byte"])
			fields[11] = exportJSONValue(citation["text_sha256"])
			fields[12] = exportJSONValue(row["text"])
		}
		for i := range fields {
			fields[i] = spreadsheetText(fields[i])
		}
		err = c.Write(fields)
		c.Flush()
		if err == nil {
			err = c.Error()
		}
	case "markdown":
		if kind == "change" {
			var row map[string]json.RawMessage
			_ = json.Unmarshal(raw, &row)
			text := exportJSONValue(row["text"])
			delete(row, "text")
			metadata, _ := json.MarshalIndent(row, "", "  ")
			_, err = w.Write([]byte("## Change · " + exportJSONValue(row["side"]) + "\n\n" + exportFence(text, "") + exportFence(string(metadata), "json")))
		} else {
			var pretty bytes.Buffer
			_ = json.Indent(&pretty, raw, "", "  ")
			_, err = w.Write([]byte("## " + kind + "\n\n" + exportFence(pretty.String(), "json")))
		}
	}
	w.records++
	return err
}
func exportFence(text, language string) string {
	// A fence longer than any run cannot be closed by untrusted source text.
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	return fence + language + "\n" + text + "\n" + fence + "\n\n"
}
func (w *analysisExportWriter) finish() (string, error) {
	if w.format == "json" {
		if _, err := w.Write([]byte("]\n")); err != nil {
			return "", err
		}
	}
	return w.data.String(), nil
}
func exportJSONValue(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}
func spreadsheetText(value string) string {
	// CSV quoting alone does not prevent a spreadsheet evaluating source formulas.
	// Prefix risky cells for display; metadata_json preserves their exact bytes.
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) || strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return "'" + value
	}
	return value
}
