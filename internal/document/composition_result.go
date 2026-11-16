package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type CompositionSection struct {
	Text   string   `json:"text"`
	Inputs []string `json:"inputs"`
}
type CompositionGap struct {
	Input string `json:"input"`
	Code  string `json:"code"`
}
type CompositionResult struct {
	Sections               []CompositionSection `json:"sections"`
	Gaps                   []CompositionGap     `json:"gaps"`
	SemanticReviewRequired bool                 `json:"semantic_review_required"`
}
type CompositionReceipt struct {
	Method         string             `json:"method"`
	Protocol       string             `json:"protocol"`
	PlanID         string             `json:"plan_id"`
	TaskID         string             `json:"task_id"`
	Ordinal        int                `json:"ordinal"`
	ModelIdentity  string             `json:"model_identity"`
	PromptTokens   int                `json:"prompt_tokens"`
	ResponseSHA256 string             `json:"response_sha256"`
	Result         *CompositionResult `json:"result"`
}
type CompositionExecution struct {
	Receipt     *CompositionReceipt `json:"receipt"`
	RawResponse []byte              `json:"-"`
}

func (c *Composition) inputs(task *CompositionTask) (map[string]CompositionSection, []string, error) {
	if task == nil || task.Ordinal < 0 || task.Ordinal >= len(c.plan.Tasks) || !reflect.DeepEqual(*task, c.plan.Tasks[task.Ordinal]) {
		return nil, nil, errors.New("invalid composition task")
	}
	items := map[string]CompositionSection{}
	order := []string{}
	for _, id := range task.Inputs {
		if seed, ok := c.seeds[id]; ok {
			if seed.Decision == "exclude" {
				return nil, nil, errors.New("excluded input in composition")
			}
			items[id] = CompositionSection{Text: seed.Candidate.Text, Inputs: []string{id}}
			order = append(order, id)
			continue
		}
		r, ok := c.receipts[id]
		if !ok {
			return nil, nil, errors.New("composition dependency is not committed")
		}
		for i, s := range r.Result.Sections {
			key := id + ":" + strconv.Itoa(i)
			items[key] = s
			order = append(order, key)
		}
	}
	return items, order, nil
}

// Resolve lineage through validated intermediate sections; never use rewritten
// model numbers as the evidence for later model numbers. Each tree edge is
// visited at most once, and references to excluded/uncommitted leaves fail.
func (c *Composition) contributors(ids []string) ([]string, error) {
	seen := map[string]bool{}
	leaves := map[string]bool{}
	var visit func(string, int) error
	visit = func(id string, depth int) error {
		if depth > 20 {
			return errors.New("composition lineage limit")
		}
		if seen[id] {
			return nil
		}
		seen[id] = true
		if s, ok := c.seeds[id]; ok {
			if s.Decision == "exclude" {
				return errors.New("excluded composition source")
			}
			leaves[id] = true
			return nil
		}
		at := strings.LastIndexByte(id, ':')
		if at < 0 {
			return errors.New("invalid composition source reference")
		}
		index, err := strconv.Atoi(id[at+1:])
		r, ok := c.receipts[id[:at]]
		if err != nil || !ok || index < 0 || index >= len(r.Result.Sections) || id != id[:at]+":"+strconv.Itoa(index) {
			return errors.New("unknown composition source reference")
		}
		for _, ref := range r.Result.Sections[index].Inputs {
			if err = visit(ref, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range ids {
		if err := visit(id, 0); err != nil {
			return nil, err
		}
	}
	// Use original source traversal order, not unstable map iteration.
	result := []string{}
	for id := range leaves {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return c.leafOrder[result[i]] < c.leafOrder[result[j]] })
	return result, nil
}

func (c *Composition) originalEvidence(ids []string) (string, error) {
	leaves, err := c.contributors(ids)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	seen := map[string]bool{}
	for _, id := range leaves {
		candidate := c.seeds[id].Candidate
		for _, group := range [][]SynthesisCitedQuote{candidate.Evidence, candidate.Conditions, candidate.Exceptions} {
			for _, q := range group {
				key := compositionDigest(q.Citation)
				if seen[key] {
					continue
				}
				seen[key] = true
				out.WriteString(q.Text)
				out.WriteByte('\n')
			}
		}
	}
	return out.String(), nil
}

func (c *Composition) Compile(task *CompositionTask, raw []byte) (*CompositionResult, error) {
	_, order, err := c.inputs(task)
	if err != nil {
		return nil, err
	}
	if len(raw) > 64<<10 || !utf8.Valid(raw) || !uniqueJSONKeys(raw) {
		return nil, errors.New("invalid composition response envelope")
	}
	var draft struct {
		TaskID   string               `json:"task_id"`
		Sections []CompositionSection `json:"sections"`
		Omitted  []string             `json:"omitted"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&draft) != nil || d.Decode(new(any)) != io.EOF || draft.TaskID != task.ID || draft.Sections == nil || len(draft.Sections) > 4 || draft.Omitted == nil || len(draft.Omitted) > len(order) {
		return nil, errors.New("invalid composition response contract")
	}
	result := &CompositionResult{Sections: []CompositionSection{}, Gaps: []CompositionGap{}, SemanticReviewRequired: true}
	used := map[string]bool{}
	omitted := map[string]bool{}
	aliases := map[string]string{}
	for i, id := range order {
		aliases["i"+strconv.Itoa(i)] = id
	}
	total := 0
	for _, section := range draft.Sections {
		total += len(section.Text)
		if strings.TrimSpace(section.Text) == "" || len(section.Text) > 2048 || total > 6144 || len(section.Inputs) == 0 || len(section.Inputs) > len(order) {
			return nil, errors.New("invalid composition section")
		}
		local := map[string]bool{}
		for i, alias := range section.Inputs {
			id, ok := aliases[alias]
			if !ok || local[id] {
				return nil, errors.New("invalid composition input citation")
			}
			local[id] = true
			used[id] = true
			section.Inputs[i] = id
		}
		evidence, err := c.originalEvidence(section.Inputs)
		if err != nil {
			return nil, err
		}
		if !synthesisNumbersSupported(section.Text, evidence) {
			for _, id := range section.Inputs {
				result.Gaps = append(result.Gaps, CompositionGap{id, "composition_numeric_mismatch"})
			}
			continue
		}
		result.Sections = append(result.Sections, section)
	}
	for _, alias := range draft.Omitted {
		id, ok := aliases[alias]
		if !ok || omitted[id] || used[id] {
			return nil, errors.New("conflicting composition omission")
		}
		omitted[id] = true
		result.Gaps = append(result.Gaps, CompositionGap{id, "composition_omitted_input"})
	}
	for _, id := range order {
		if !used[id] && !omitted[id] {
			return nil, errors.New("unaccounted composition input")
		}
	}
	return result, nil
}

// Sections returns bounded final prose and original candidate references. A
// caller opens each candidate's exact quotations through the parent source API.
// Human approval is not inherited by newly generated composition prose.
type ComposedSection struct {
	Text         string            `json:"text"`
	Contributors []CompositionLeaf `json:"contributors"`
	Verification string            `json:"verification"`
}

type CompositionReport struct {
	PlanID                 string            `json:"plan_id"`
	SourcePlanID           string            `json:"source_plan_id"`
	SourceUnits            int               `json:"source_units"`
	Sections               []ComposedSection `json:"sections"`
	Gaps                   []CompositionGap  `json:"gaps"`
	ExtractionGaps         []AnalysisGap     `json:"extraction_gaps"`
	SemanticReviewRequired bool              `json:"semantic_review_required"`
}

func (c *Composition) Report() (*CompositionReport, error) {
	sections, gaps, err := c.Sections()
	if err != nil {
		return nil, err
	}
	return &CompositionReport{PlanID: c.plan.ID, SourcePlanID: c.plan.SourcePlanID, SourceUnits: c.sourceUnits, Sections: sections, Gaps: gaps, ExtractionGaps: append([]AnalysisGap{}, c.sourceGaps...), SemanticReviewRequired: true}, nil
}

func (c *Composition) Sections() ([]ComposedSection, []CompositionGap, error) {
	if len(c.receipts) != len(c.plan.Tasks) {
		return nil, nil, errors.New("composition traversal unfinished")
	}
	sections := []ComposedSection{}
	gaps := []CompositionGap{}
	for _, leaf := range c.plan.Leaves {
		if leaf.Decision == "unreviewed" {
			gaps = append(gaps, CompositionGap{leaf.ID, "composition_input_review_pending"})
		}
	}
	for _, task := range c.plan.Tasks {
		gaps = append(gaps, c.receipts[task.ID].Result.Gaps...)
	}
	if c.plan.Root == "" {
		return sections, append(gaps, CompositionGap{"", "composition_no_selected_candidates"}), nil
	}
	for _, section := range c.receipts[c.plan.Root].Result.Sections {
		ids, err := c.contributors(section.Inputs)
		if err != nil {
			return nil, nil, err
		}
		refs := []CompositionLeaf{}
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		for _, leaf := range c.plan.Leaves {
			if set[leaf.ID] {
				refs = append(refs, leaf)
			}
		}
		sections = append(sections, ComposedSection{Text: section.Text, Contributors: refs, Verification: "source_identity_only"})
	}
	return sections, gaps, nil
}
