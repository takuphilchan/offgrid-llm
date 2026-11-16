package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const CompositionProtocol = "document-composition-v1"

// Storage must verify each seed against the parent's retained model receipt and
// the pinned review revision. This core additionally verifies original quotes.
type CompositionSeed struct {
	Batch          int                `json:"batch"`
	BatchID        string             `json:"batch_id"`
	CandidateIndex int                `json:"candidate_index"`
	Candidate      SynthesisCandidate `json:"candidate"`
	ReviewRevision int                `json:"review_revision"`
	Decision       string             `json:"decision"`
	SectionChecked bool               `json:"section_checked"`
}
type CompositionLeaf struct {
	ID              string `json:"id"`
	Batch           int    `json:"batch"`
	BatchID         string `json:"batch_id"`
	CandidateIndex  int    `json:"candidate_index"`
	CandidateSHA256 string `json:"candidate_sha256"`
	ReviewRevision  int    `json:"review_revision"`
	Decision        string `json:"decision"`
	SectionChecked  bool   `json:"section_checked"`
}
type CompositionTask struct {
	ID      string   `json:"id"`
	Ordinal int      `json:"ordinal"`
	Inputs  []string `json:"inputs"`
}
type CompositionPlan struct {
	ID           string            `json:"id"`
	Protocol     string            `json:"protocol"`
	AnalysisID   string            `json:"analysis_id"`
	SourcePlanID string            `json:"source_plan_id"`
	Leaves       []CompositionLeaf `json:"leaves"`
	Tasks        []CompositionTask `json:"tasks"`
	Root         string            `json:"root"`
}

func compositionDigest(value any) string {
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

// The tree accounts for every selected candidate. It contains at most N merge
// tasks and logarithmic depth; no prompt contains an unbounded list of drafts.
func BuildCompositionPlan(analysisID string, sourcePlan *AnalysisPlan, seeds []CompositionSeed) (*CompositionPlan, error) {
	if analysisID == "" || len(analysisID) > 128 || sourcePlan == nil || sourcePlan.Version != AnalysisVersion || sourcePlan.ID == "" || sourcePlan.Workflow != "summary" || len(seeds) > 65536 {
		return nil, errors.New("invalid composition source plan")
	}
	p := &CompositionPlan{Protocol: CompositionProtocol, AnalysisID: analysisID, SourcePlanID: sourcePlan.ID, Leaves: []CompositionLeaf{}, Tasks: []CompositionTask{}}
	seen := map[string]bool{}
	bytes := 0
	for _, s := range seeds {
		key := strconv.Itoa(s.Batch) + ":" + strconv.Itoa(s.CandidateIndex)
		if s.Batch < 0 || s.Batch > 100000 || s.BatchID == "" || len(s.BatchID) > 128 || s.CandidateIndex < 0 || s.CandidateIndex >= 32 || seen[key] || s.ReviewRevision < 0 || s.ReviewRevision > 256 || !oneOf(s.Decision, "keep", "exclude", "unreviewed") || s.SectionChecked && s.Decision == "unreviewed" || s.ReviewRevision == 0 && (s.Decision != "unreviewed" || s.SectionChecked) {
			return nil, errors.New("invalid composition seed identity")
		}
		seen[key] = true
		if !utf8.ValidString(s.Candidate.Text) || strings.TrimSpace(s.Candidate.Text) == "" || len(s.Candidate.Text) > 4096 || s.Candidate.Verification != "source_identity_only" || len(s.Candidate.Evidence) == 0 || len(s.Candidate.Evidence) > 8 || s.Candidate.Conditions == nil || len(s.Candidate.Conditions) > 8 || s.Candidate.Exceptions == nil || len(s.Candidate.Exceptions) > 8 {
			return nil, errors.New("invalid composition candidate")
		}
		raw, _ := json.Marshal(s.Candidate)
		bytes += len(raw)
		if bytes > 64<<20 {
			return nil, errors.New("composition input limit exceeded")
		}
		leaf := CompositionLeaf{Batch: s.Batch, BatchID: s.BatchID, CandidateIndex: s.CandidateIndex, CandidateSHA256: compositionDigest(s.Candidate), ReviewRevision: s.ReviewRevision, Decision: s.Decision, SectionChecked: s.SectionChecked}
		leaf.ID = "leaf-" + compositionDigest(leaf)
		p.Leaves = append(p.Leaves, leaf)
	}
	p.ID = "composition-" + compositionDigest(p)
	var level []string
	for _, leaf := range p.Leaves {
		if leaf.Decision != "exclude" {
			level = append(level, leaf.ID)
		}
	}
	for len(level) > 1 || len(level) == 1 && len(p.Tasks) == 0 {
		next := []string{}
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) && len(level) > 1 {
				next = append(next, level[i])
				continue
			}
			inputs := append([]string{}, level[i:min(i+2, len(level))]...)
			task := CompositionTask{Ordinal: len(p.Tasks), Inputs: inputs}
			task.ID = "merge-" + compositionDigest(struct {
				Plan string
				Task CompositionTask
			}{p.ID, task})
			p.Tasks = append(p.Tasks, task)
			next = append(next, task.ID)
		}
		level = next
	}
	if len(level) == 1 {
		p.Root = level[0]
	}
	return p, nil
}

type Composition struct {
	plan        *CompositionPlan
	seeds       map[string]CompositionSeed
	receipts    map[string]*CompositionReceipt
	leafOrder   map[string]int
	sourceGaps  []AnalysisGap
	sourceUnits int
}

func PrepareComposition(plan *CompositionPlan, sourcePlan *AnalysisPlan, graphs []*Extraction, seeds []CompositionSeed) (*Composition, error) {
	if plan == nil {
		return nil, errors.New("missing composition plan")
	}
	if _, err := PrepareSynthesis(sourcePlan, graphs); err != nil {
		return nil, err
	}
	expected, err := BuildCompositionPlan(plan.AnalysisID, sourcePlan, seeds)
	if err != nil || !reflect.DeepEqual(expected, plan) {
		return nil, errors.New("composition plan does not match immutable seeds")
	}
	c := &Composition{plan: expected, seeds: map[string]CompositionSeed{}, receipts: map[string]*CompositionReceipt{}, leafOrder: map[string]int{}, sourceGaps: append([]AnalysisGap{}, sourcePlan.Gaps...), sourceUnits: len(sourcePlan.Units)}
	verifyQuote := compositionQuoteVerifier(graphs)
	for i, s := range seeds {
		var quoted strings.Builder
		for _, group := range [][]SynthesisCitedQuote{s.Candidate.Evidence, s.Candidate.Conditions, s.Candidate.Exceptions} {
			for _, q := range group {
				if !verifyQuote(q) {
					return nil, errors.New("invalid composition source citation")
				}
				quoted.WriteString(q.Text)
				quoted.WriteByte('\n')
			}
		}
		if !synthesisNumbersSupported(s.Candidate.Text, quoted.String()) {
			return nil, errors.New("composition seed has unsupported numeric values")
		}
		// Deep-copy inputs; a caller editing its draft must not mutate a running tree.
		raw, _ := json.Marshal(s)
		var cloned CompositionSeed
		_ = json.Unmarshal(raw, &cloned)
		c.seeds[expected.Leaves[i].ID] = cloned
		c.leafOrder[expected.Leaves[i].ID] = i
	}
	return c, nil
}

func (c *Composition) Next() *CompositionTask {
	if len(c.receipts) == len(c.plan.Tasks) {
		return nil
	}
	t := c.plan.Tasks[len(c.receipts)]
	t.Inputs = append([]string{}, t.Inputs...)
	return &t
}
