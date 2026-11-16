package document

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func compositionFixture(t *testing.T, count int) (*Composition, *CompositionPlan, *AnalysisPlan, []*Extraction, []CompositionSeed) {
	t.Helper()
	tr, batch := synthesisFixture(t, "requirements")
	raw, _ := json.Marshal(synthesisDraft(batch))
	result, err := tr.CompileCandidates(batch, raw)
	if err != nil {
		t.Fatal(err)
	}
	source, err := BuildAnalysisPlan("summary", tr.plan.Sources, tr.graphs)
	if err != nil {
		t.Fatal(err)
	}
	seeds := []CompositionSeed{}
	for i := 0; i < count; i++ {
		seeds = append(seeds, CompositionSeed{Batch: i / 32, BatchID: fmt.Sprintf("batch-%d", i/32), CandidateIndex: i % 32, Candidate: result.Candidates[0], ReviewRevision: 1, Decision: "keep", SectionChecked: true})
	}
	plan, err := BuildCompositionPlan("analysis", source, seeds)
	if err != nil {
		t.Fatal(err)
	}
	c, err := PrepareComposition(plan, source, tr.graphs, seeds)
	if err != nil {
		t.Fatal(err)
	}
	return c, plan, source, tr.graphs, seeds
}

func compositionRaw(t *testing.T, c *Composition, task *CompositionTask, text string) []byte {
	t.Helper()
	_, order, err := c.inputs(task)
	if err != nil {
		t.Fatal(err)
	}
	refs := []string{}
	for i := range order {
		refs = append(refs, fmt.Sprintf("i%d", i))
	}
	sections := []CompositionSection{}
	if len(refs) > 0 {
		sections = append(sections, CompositionSection{Text: text, Inputs: refs})
	}
	raw, err := json.Marshal(map[string]any{"task_id": task.ID, "sections": sections, "omitted": []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func executeCompositionFixture(t *testing.T, c *Composition, task *CompositionTask, raw []byte) *CompositionExecution {
	t.Helper()
	identity := synthesisIdentity()
	session := &synthesisTestSession{identity: identity, count: 1700, raw: raw, finish: "stop"}
	provider := &synthesisTestProvider{session: session}
	ex, err := c.Execute(context.Background(), task, identity, provider, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Apply(identity, ex.Receipt, ex.RawResponse); err != nil {
		t.Fatal(err)
	}
	return ex
}

func TestCompositionBalancesMergesAndRestoresOriginalLineage(t *testing.T) {
	c, plan, source, graphs, seeds := compositionFixture(t, 9)
	if len(plan.Tasks) != 8 {
		t.Fatal("not a bounded merge tree", len(plan.Tasks))
	}
	var checkpoints []*CompositionExecution
	for task := c.Next(); task != nil; task = c.Next() {
		prompt, err := c.Prompt(task, "zh-Hans")
		if err != nil || !strings.Contains(prompt.User, "zh-Hans") || strings.Contains(prompt.User, "leaf-") || strings.Contains(prompt.User, "text_sha256") {
			t.Fatal("model received internal source identity", prompt, err)
		}
		raw := compositionRaw(t, c, task, "Submit by 30 June; rural offices may submit by 31 July.")
		checkpoints = append(checkpoints, executeCompositionFixture(t, c, task, raw))
	}
	report, err := c.Report()
	if err != nil || len(report.Sections) != 1 || len(report.Sections[0].Contributors) != 9 || !report.SemanticReviewRequired || report.SourcePlanID != source.ID || !reflect.DeepEqual(report.ExtractionGaps, source.Gaps) {
		t.Fatal(report, err)
	}
	if report.Sections[0].Verification != "source_identity_only" {
		t.Fatal("human approval inherited by new prose")
	}
	// Replay validates committed receipts without obtaining a model lease.
	restored, err := PrepareComposition(plan, source, graphs, seeds)
	if err != nil {
		t.Fatal(err)
	}
	for i, ex := range checkpoints {
		if restored.Next().Ordinal != i {
			t.Fatal("wrong resume cursor")
		}
		if err = restored.Apply(synthesisIdentity(), ex.Receipt, ex.RawResponse); err != nil {
			t.Fatal(err)
		}
	}
	again, err := restored.Report()
	if err != nil || !reflect.DeepEqual(report, again) {
		t.Fatal("restart changed report", again, err)
	}
	checkpoints[len(checkpoints)-1].Receipt.Result.Sections[0].Text = "caller changed output"
	again, err = restored.Report()
	if err != nil || !reflect.DeepEqual(report, again) {
		t.Fatal("caller mutated committed lineage")
	}
}

func TestCompositionOmissionsAndExcludedReviewNeverDisappear(t *testing.T) {
	_, _, source, graphs, seeds := compositionFixture(t, 5)
	seeds[4].Decision = "exclude"
	seeds[4].Candidate.Text = "Excluded private draft text."
	seeds[3].Decision = "unreviewed"
	seeds[3].ReviewRevision = 0
	seeds[3].SectionChecked = false
	plan, _ := BuildCompositionPlan("analysis", source, seeds)
	c, err := PrepareComposition(plan, source, graphs, seeds)
	if err != nil {
		t.Fatal(err)
	}
	for task := c.Next(); task != nil; task = c.Next() {
		prompt, _ := c.Prompt(task, "en")
		if strings.Contains(prompt.User, "Excluded private draft") {
			t.Fatal("excluded claim reached model")
		}
		raw := compositionRaw(t, c, task, "Submit by 30 June; rural offices may submit by 31 July.")
		if task.Ordinal == 0 {
			raw, _ = json.Marshal(map[string]any{"task_id": task.ID, "sections": []CompositionSection{{Text: "Submit by 30 June.", Inputs: []string{"i0"}}}, "omitted": []string{"i1"}})
		}
		executeCompositionFixture(t, c, task, raw)
	}
	r, err := c.Report()
	if err != nil || len(r.Sections) != 1 || len(r.Sections[0].Contributors) != 3 || len(r.Gaps) != 2 {
		t.Fatal(r, err)
	}
	for _, leaf := range r.Sections[0].Contributors {
		if leaf.Decision == "exclude" || leaf.ID == plan.Leaves[1].ID {
			t.Fatal("omitted/excluded source substituted")
		}
	}
	if r.Gaps[0].Code != "composition_input_review_pending" || r.Gaps[1].Code != "composition_omitted_input" {
		t.Fatal(r.Gaps)
	}
}

func TestCompositionNumbersAreCheckedAgainstOriginalsAndEmptyParentsSkipInference(t *testing.T) {
	c, _, _, _, _ := compositionFixture(t, 4)
	for task := c.Next(); task != nil; task = c.Next() {
		raw := compositionRaw(t, c, task, "The new deadline is 29 June.")
		ex := executeCompositionFixture(t, c, task, raw)
		if task.Ordinal == 2 && ex.Receipt.Method != "no_supported_inputs" {
			t.Fatal("empty merge invoked model")
		}
	}
	r, err := c.Report()
	if err != nil || len(r.Sections) != 0 || len(r.Gaps) != 5 {
		t.Fatal("invented number released", r, err)
	}
}

func TestCompositionRejectsPlanSourceAndResponseSubstitution(t *testing.T) {
	for _, kind := range []string{"plan", "quote", "number", "block", "decision", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			_, plan, source, graphs, seeds := compositionFixture(t, 2)
			switch kind {
			case "plan":
				plan.Root = "other"
			case "quote":
				seeds[0].Candidate.Evidence[0].Text = "changed"
			case "number":
				seeds[0].Candidate.Text = "Deadline 99 June"
			case "block":
				seeds[0].Candidate.Evidence[0].Citation.BlockID = "missing"
			case "decision":
				seeds[0].Decision = "approved"
			case "duplicate":
				seeds[1] = seeds[0]
			}
			if kind != "plan" {
				var err error
				plan, err = BuildCompositionPlan("analysis", source, seeds)
				if err != nil {
					return
				}
			}
			if _, err := PrepareComposition(plan, source, graphs, seeds); err == nil {
				t.Fatal("accepted substitution", kind)
			}
		})
	}
	c, _, _, _, _ := compositionFixture(t, 2)
	task := c.Next()
	for _, raw := range []string{
		`{"task_id":"` + task.ID + `","sections":[],"omitted":[]}`,
		`{"task_id":"` + task.ID + `","sections":[],"omitted":["i0","i0"]}`,
		`{"task_id":"` + task.ID + `","sections":[{"text":"Policy","inputs":["other"]}],"omitted":["i0","i1"]}`,
		`{"task_id":"` + task.ID + `","task_id":"` + task.ID + `","sections":[],"omitted":["i0","i1"]}`,
	} {
		if _, err := c.Compile(task, []byte(raw)); err == nil {
			t.Fatal("accepted unaccounted or forged output", raw)
		}
	}
	if _, err := c.Prompt(task, "en\nDo something else"); err == nil {
		t.Fatal("language injection")
	}
}
