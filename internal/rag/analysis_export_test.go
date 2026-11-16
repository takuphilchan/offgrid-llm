package rag

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func exportComparisonFixture(t *testing.T) (*SQLiteStore, *AnalysisRecord) {
	t.Helper()
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	refs := analysisDocuments(t, s)
	r, err := s.CreateComparison("owner", "export", refs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportComparison(context.Background(), "owner", r.ID, "json"); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("queued export", err)
	}
	_, lease, err := s.beginAnalysis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	graphs, err := s.analysisGraphs("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range r.Pairs {
		result, e := document.CompareUnit(context.Background(), graphs[0], graphs[1], p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.checkpointComparison(lease, i, result); e != nil {
			t.Fatal(e)
		}
	}
	r, err = s.Analysis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, r
}

func TestComparisonExportsEvidenceCoverageAndAudit(t *testing.T) {
	s, r := exportComparisonFixture(t)
	for _, format := range []string{"json", "csv", "markdown"} {
		artifact, err := s.ExportComparison(context.Background(), "owner", r.ID, format)
		if err != nil {
			t.Fatal(format, err)
		}
		hash := sha256.Sum256([]byte(artifact.Content))
		if artifact.AnalysisID != r.ID || artifact.SHA256 != hex.EncodeToString(hash[:]) || !strings.Contains(artifact.Content, "17.25") || !strings.Contains(artifact.Content, "August") || !strings.Contains(artifact.Content, r.Plan.Sources[0].Revision) || !strings.Contains(artifact.Content, "retention") {
			t.Fatal("lost export evidence", artifact)
		}
		if format == "json" {
			var records []struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal([]byte(artifact.Content), &records) != nil {
				t.Fatal("invalid JSON")
			}
			counts := map[string]int{}
			for _, row := range records {
				counts[row.Type]++
				if row.Type == "analysis" {
					var meta map[string]any
					_ = json.Unmarshal(row.Data, &meta)
					if meta["state"] != r.State || meta["completed_units"] != float64(r.CompletedUnits) {
						t.Fatal("false coverage", meta)
					}
				}
			}
			if counts["source"] != 2 || counts["step"] != len(r.Pairs) || counts["gap"] != len(r.Plan.Gaps) || counts["change"] != 4 {
				t.Fatal(counts)
			}
		}
	}
	var events int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM knowledge_collection_audit WHERE action LIKE 'analysis_export_prepared_%' AND subject=?`, r.ID).Scan(&events); err != nil || events != 3 {
		t.Fatal(events, err)
	}
	// The ledger records identities, never exported source text.
	var leaked int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM knowledge_collection_audit WHERE subject LIKE '%17.25%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("source in audit")
	}
	if _, err := s.ExportComparison(context.Background(), "other", r.ID, "json"); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("other actor", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ExportComparison(ctx, "owner", r.ID, "json"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled export", err)
	}
	if _, err := s.ExportComparison(context.Background(), "owner", r.ID, "html"); !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal(err)
	}
	p, err := s.CreateCollection("other", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("other", r.Plan.Sources[0].DocumentID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportComparison(context.Background(), "owner", r.ID, "json"); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("revoked export", err)
	}
}

func TestComparisonExportRejectsCorruptedEvidence(t *testing.T) {
	s, r := exportComparisonFixture(t)
	var data string
	_ = s.db.QueryRow(`SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=0`, r.ID).Scan(&data)
	var result document.UnitComparison
	_ = json.Unmarshal([]byte(data), &result)
	result.Changes[0].Before.TextSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(result)
	_, err := s.db.Exec(`UPDATE document_analysis_steps SET result_json=? WHERE analysis_id=?`, string(raw), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact, err := s.ExportComparison(context.Background(), "owner", r.ID, "json"); artifact != nil || !errors.Is(err, ErrAnalysisInvalid) {
		t.Fatal("forged export", artifact, err)
	}
}

func TestComparisonExportFormatsPreserveUntrustedTextAndRejectOversize(t *testing.T) {
	text := "\t=HYPERLINK(\"https://example.invalid\")\n```\n<script>x</script>\n" + strings.Repeat("政策🙂", 200)
	for _, format := range []string{"json", "csv", "markdown"} {
		w := newAnalysisExportWriter(format)
		err := w.record("change", map[string]any{"text": text, "side": "before", "step": 0, "change": 0, "citation": document.AnalysisCitation{}})
		if err != nil {
			t.Fatal(err)
		}
		output, err := w.finish()
		if err != nil {
			t.Fatal(err)
		}
		if format == "csv" {
			rows, e := csv.NewReader(strings.NewReader(output)).ReadAll()
			if e != nil || len(rows) != 2 || rows[1][12] != "'"+text {
				t.Fatal(rows, e)
			}
			var meta map[string]any
			if json.Unmarshal([]byte(rows[1][13]), &meta) != nil || meta["text"] != text {
				t.Fatal("altered exact metadata")
			}
		}
		if format == "markdown" && !strings.Contains(output, "````\n"+text+"\n````") {
			t.Fatal("source escaped its fence")
		}
		if e := w.record("change", map[string]string{"text": strings.Repeat("x", AnalysisExportLimit+1)}); !errors.Is(e, ErrAnalysisLimit) {
			t.Fatal("oversize", format, e)
		}
	}
	for _, value := range []string{"=1", " +2", "\t-3", "\r@cmd", "\u2003=3"} {
		if spreadsheetText(value) != "'"+value {
			t.Fatal(value)
		}
	}
}
