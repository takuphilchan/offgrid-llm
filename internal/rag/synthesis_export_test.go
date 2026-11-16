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
)

func completedSynthesisExport(t *testing.T, s *SQLiteStore) *SynthesisRecord {
	t.Helper()
	r, err := s.CreateSynthesis("owner", "export", "requirements", synthesisTestDocument(t, s, "=SUM(17,25)\n政策: Exception July.\n```\nUntrusted text."), synthesisTestIdentity(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportSynthesis(context.Background(), "owner", r.ID, "json"); !errors.Is(err, ErrAnalysisConflict) {
		t.Fatal("unfinished export", err)
	}
	r, lease, err := s.beginSynthesis("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	for r.State == "running" {
		tr, b, execution := synthesisTestBatch(t, s, r)
		if err = s.checkpointSynthesis(lease, r.CompletedBatches, tr, b, execution); err != nil {
			t.Fatal(err)
		}
		r, err = s.Synthesis("owner", r.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestSynthesisExportEvidenceCoverageAndFormats(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := completedSynthesisExport(t, s)
	for _, format := range []string{"json", "csv", "markdown"} {
		t.Run(format, func(t *testing.T) {
			out, err := s.ExportSynthesis(context.Background(), "owner", r.ID, format)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(out.Content))
			if out.AnalysisID != r.ID || out.Format != format || out.SHA256 != hex.EncodeToString(hash[:]) {
				t.Fatal("wrong artifact receipt")
			}
			for _, want := range []string{"awaiting_review", "source_identity_only", "semantic_review_required", "政策", r.Plan.Sources[0].Revision, "text_sha256", "conditions", "exceptions", "separate retention"} {
				if !strings.Contains(out.Content, want) {
					t.Fatalf("missing %s", want)
				}
			}
			if strings.Contains(out.Content, "raw_response") {
				t.Fatal("private raw response exported")
			}
			if format == "json" {
				var records []struct {
					Type string
					Data json.RawMessage
				}
				if json.Unmarshal([]byte(out.Content), &records) != nil || records[0].Type != "analysis" {
					t.Fatal("invalid JSON records")
				}
			}
			if format == "csv" {
				rows, err := csv.NewReader(strings.NewReader(out.Content)).ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				candidates := 0
				for _, row := range rows[1:] {
					if row[0] != "candidate" {
						continue
					}
					candidates++
					if !strings.HasPrefix(row[3], "'=SUM(") || !strings.HasPrefix(row[4], "'=SUM(") || !strings.Contains(row[8], `"text":"=SUM(`) {
						t.Fatal("spreadsheet protection lost original")
					}
				}
				if candidates != 1 {
					t.Fatal(candidates)
				}
			}
			if format == "markdown" && !strings.Contains(out.Content, "````\n=SUM(") {
				t.Fatal("source fence injection")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := s.ExportSynthesis(ctx, "owner", r.ID, "json"); err == nil || out != nil {
		t.Fatal("cancelled artifact delivered")
	}
	if _, err = s.ExportSynthesis(context.Background(), "other", r.ID, "json"); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("cross-owner export", err)
	}
	p, err := s.CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", "source", p.ID, 1); err != nil {
		t.Fatal(err)
	}
	// Directly revoke the job owner's source audience to simulate an authorized
	// collection ownership transfer independently of private job ownership.
	if _, err = s.db.Exec(`UPDATE knowledge_collections SET owner='another' WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExportSynthesis(context.Background(), "owner", r.ID, "json"); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("revoked source exported", err)
	}
}

func TestSynthesisExportRejectsTamperedTraversal(t *testing.T) {
	for _, mode := range []string{"candidate", "missing", "extra", "cursor"} {
		t.Run(mode, func(t *testing.T) {
			s, err := NewSQLiteStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := completedSynthesisExport(t, s)
			switch mode {
			case "candidate":
				var raw string
				if err = s.db.QueryRow(`SELECT result_json FROM document_analysis_steps WHERE analysis_id=? AND ordinal=0`, r.ID).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				var checkpoint synthesisCheckpoint
				if err = json.Unmarshal([]byte(raw), &checkpoint); err != nil {
					t.Fatal(err)
				}
				checkpoint.Receipt.Result.Candidates[0].Text = "Everything approved"
				data, _ := json.Marshal(checkpoint)
				_, err = s.db.Exec(`UPDATE document_analysis_steps SET result_json=? WHERE analysis_id=?`, string(data), r.ID)
			case "missing":
				_, err = s.db.Exec(`DELETE FROM document_analysis_steps WHERE analysis_id=?`, r.ID)
			case "extra":
				_, err = s.db.Exec(`INSERT INTO document_analysis_steps SELECT analysis_id,42,result_json,storage_bytes FROM document_analysis_steps WHERE analysis_id=?`, r.ID)
			case "cursor":
				_, err = s.db.Exec(`UPDATE document_synthesis_configs SET next_byte=1 WHERE analysis_id=?`, r.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if out, err := s.ExportSynthesis(context.Background(), "owner", r.ID, "json"); err == nil || out != nil {
				t.Fatal("tampered artifact delivered")
			}
		})
	}
}

func TestSynthesisExportLimitIsAllOrNothing(t *testing.T) {
	for _, format := range []string{"json", "csv", "markdown"} {
		w := newSynthesisExportWriter(format)
		if err := w.record("gap", strings.Repeat("x", AnalysisExportLimit)); !errors.Is(err, ErrAnalysisLimit) {
			t.Fatal(format, err)
		}
	}
}
