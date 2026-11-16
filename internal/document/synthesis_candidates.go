package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

type SynthesisQuote struct {
	EvidenceID string `json:"evidence_id"`
	Quote      string `json:"quote"`
}
type SynthesisItem struct {
	Text       string           `json:"text"`
	Evidence   []SynthesisQuote `json:"evidence"`
	Conditions []SynthesisQuote `json:"conditions"`
	Exceptions []SynthesisQuote `json:"exceptions"`
}
type SynthesisDraft struct {
	BatchID          string          `json:"batch_id"`
	ReviewedEvidence []string        `json:"reviewed_evidence"`
	Items            []SynthesisItem `json:"items"`
	NoFindings       bool            `json:"no_findings"`
}
type SynthesisCitedQuote struct {
	Text     string           `json:"text"`
	Citation AnalysisCitation `json:"citation"`
}
type SynthesisCandidate struct {
	Text         string                `json:"text"`
	Evidence     []SynthesisCitedQuote `json:"evidence"`
	Conditions   []SynthesisCitedQuote `json:"conditions"`
	Exceptions   []SynthesisCitedQuote `json:"exceptions"`
	Verification string                `json:"verification"`
}
type SynthesisCandidates struct {
	BatchID                string               `json:"batch_id"`
	Candidates             []SynthesisCandidate `json:"candidates"`
	Gaps                   []AnalysisGap        `json:"gaps"`
	NoFindingsReported     bool                 `json:"no_findings_reported"`
	SemanticReviewRequired bool                 `json:"semantic_review_required"`
}

// CompileCandidates establishes exact quote identity and numeric consistency,
// not semantic entailment. These are candidate sections, not approved answers.
// A runner must retain the review distinction and perform its support assessment
// before publication; a second model is not independent human verification.
func (t *SynthesisTraversal) CompileCandidates(batch *SynthesisBatch, raw []byte) (*SynthesisCandidates, error) {
	return t.compileCandidates(batch, raw, true)
}

func (t *SynthesisTraversal) compileCandidates(batch *SynthesisBatch, raw []byte, calendarDates bool) (*SynthesisCandidates, error) {
	if err := t.VerifyBatch(batch); err != nil {
		return nil, err
	}
	if len(raw) > 128<<10 || !utf8.Valid(raw) || !uniqueJSONKeys(raw) {
		return nil, errors.New("invalid synthesis response envelope")
	}
	var draft SynthesisDraft
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&draft) != nil || decoder.Decode(new(any)) != io.EOF || draft.BatchID != batch.ID || draft.Items == nil || len(draft.Items) > 32 || len(draft.ReviewedEvidence) != len(batch.Evidence) {
		return nil, errors.New("invalid synthesis response contract")
	}
	evidence := map[string]SynthesisEvidence{}
	for _, e := range batch.Evidence {
		evidence[e.ID] = e
	}
	seen := map[string]bool{}
	for _, id := range draft.ReviewedEvidence {
		if _, ok := evidence[id]; !ok || seen[id] {
			return nil, errors.New("unaccounted synthesis evidence")
		}
		seen[id] = true
	}
	if draft.NoFindings && (len(draft.Items) > 0 || batch.Workflow != "requirements") {
		return nil, errors.New("conflicting synthesis disposition")
	}
	result := &SynthesisCandidates{BatchID: batch.ID, Candidates: []SynthesisCandidate{}, Gaps: append([]AnalysisGap{}, batch.Gaps...), NoFindingsReported: draft.NoFindings, SemanticReviewRequired: true}
	unit := t.plan.Units[batch.UnitOrdinal]
	gap := func(code string) {
		result.Gaps = append(result.Gaps, AnalysisGap{Source: unit.Source, UnitID: unit.UnitID, Code: code})
	}
	if len(draft.Items) == 0 && !draft.NoFindings {
		gap("analysis_empty_model_output")
	}
	for _, item := range draft.Items {
		if strings.TrimSpace(item.Text) == "" || len(item.Text) > 4096 || len(item.Evidence) < 1 || len(item.Evidence) > 8 || item.Conditions == nil || item.Exceptions == nil || len(item.Conditions) > 8 || len(item.Exceptions) > 8 || batch.Workflow == "summary" && (len(item.Conditions) > 0 || len(item.Exceptions) > 0) {
			gap("analysis_invalid_candidate")
			continue
		}
		c := SynthesisCandidate{Text: item.Text, Verification: "source_identity_only", Evidence: []SynthesisCitedQuote{}, Conditions: []SynthesisCitedQuote{}, Exceptions: []SynthesisCitedQuote{}}
		ok, primary := true, false
		for i, quotes := range [][]SynthesisQuote{item.Evidence, item.Conditions, item.Exceptions} {
			target := []*[]SynthesisCitedQuote{&c.Evidence, &c.Conditions, &c.Exceptions}[i]
			for _, q := range quotes {
				e, found := evidence[q.EvidenceID]
				if !found || strings.TrimSpace(q.Quote) == "" || len(q.Quote) > 8192 {
					ok = false
					break
				}
				at := strings.Index(e.Text, q.Quote)
				if at < 0 || strings.Index(e.Text[at+1:], q.Quote) >= 0 {
					ok = false
					break
				}
				citation := e.Citation
				citation.Span = Span{e.Citation.Span.Start + at, e.Citation.Span.Start + at + len(q.Quote)}
				hash := sha256.Sum256([]byte(q.Quote))
				citation.TextSHA256 = hex.EncodeToString(hash[:])
				*target = append(*target, SynthesisCitedQuote{Text: q.Quote, Citation: citation})
				if i == 0 && e.Role == "primary" {
					primary = true
				}
			}
		}
		if !ok || !primary {
			gap("analysis_unsupported_citation")
			continue
		}
		var quoted strings.Builder
		for _, group := range [][]SynthesisCitedQuote{c.Evidence, c.Conditions, c.Exceptions} {
			for _, q := range group {
				quoted.WriteString(q.Text)
				quoted.WriteByte('\n')
			}
		}
		claim := item.Text
		if calendarDates {
			claim = withoutSupportedCalendarDates(claim, quoted.String())
		}
		if !synthesisLiteralNumbersSupported(claim, quoted.String()) {
			gap("analysis_numeric_mismatch")
			continue
		}
		result.Candidates = append(result.Candidates, c)
	}
	return result, nil
}

// Reject duplicate object keys and excessive nesting before struct decoding.
func uniqueJSONKeys(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var visit func(int) bool
	visit = func(depth int) bool {
		if depth > 16 {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		delimiter, container := token.(json.Delim)
		if !container {
			return true
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] || name != strings.ToLower(name) {
					return false
				}
				seen[name] = true
				if !visit(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !visit(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !visit(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}
