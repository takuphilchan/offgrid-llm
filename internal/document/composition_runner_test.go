package document

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCompositionRunnerRejectsRevocationBudgetRuntimeChangeAndReplay(t *testing.T) {
	for _, mode := range []string{"valid", "guard1", "guard2", "guard3", "guard4", "guard5", "budget", "count", "changed", "twice", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			c, _, _, _, _ := compositionFixture(t, 2)
			task := c.Next()
			identity := synthesisIdentity()
			session := &synthesisTestSession{identity: identity, count: 1700, raw: compositionRaw(t, c, task, "Submit by 30 June."), finish: "stop"}
			provider := &synthesisTestProvider{session: session}
			revoke := 0
			switch mode {
			case "guard1":
				revoke = 1
			case "guard2":
				revoke = 2
			case "guard3":
				revoke = 3
			case "guard4":
				revoke = 4
			case "guard5":
				revoke = 5
			case "budget":
				session.count = 100000
			case "count":
				session.countErr = errors.New("unavailable")
			case "changed":
				session.mutateAfterCount = true
			case "twice":
				provider.twice = true
			case "truncated":
				session.finish = "length"
			}
			guards := 0
			ex, err := c.Execute(context.Background(), task, identity, provider, func() error {
				guards++
				if guards == revoke {
					return errors.New("revoked")
				}
				return nil
			})
			if mode == "valid" {
				if err != nil || ex == nil || guards != 5 || session.counted != 1 || session.generated != 1 {
					t.Fatal(ex, err, guards, session)
				}
				return
			}
			if err == nil || ex != nil {
				t.Fatal("invalid generation released", mode, ex, err)
			}
			if c.Next().Ordinal != 0 {
				t.Fatal("uncommitted execution advanced cursor")
			}
			if session.generated > 1 {
				t.Fatal("generation replayed")
			}
		})
	}
}

func TestCompositionCheckpointRejectsTamperingAndChangedModel(t *testing.T) {
	c, plan, source, graphs, seeds := compositionFixture(t, 4)
	task := c.Next()
	ex := executeCompositionFixture(t, c, task, compositionRaw(t, c, task, "Submit by 30 June."))
	for _, mode := range []string{"text", "hash", "plan", "ordinal", "count", "model"} {
		t.Run(mode, func(t *testing.T) {
			fresh, err := PrepareComposition(plan, source, graphs, seeds)
			if err != nil {
				t.Fatal(err)
			}
			r := *ex.Receipt
			identity := synthesisIdentity()
			switch mode {
			case "text":
				copyResult := *r.Result
				copyResult.Sections = append([]CompositionSection{}, r.Result.Sections...)
				copyResult.Sections[0].Text = "Forged"
				r.Result = &copyResult
			case "hash":
				r.ResponseSHA256 = strings.Repeat("f", 64)
			case "plan":
				r.PlanID = "other"
			case "ordinal":
				r.Ordinal++
			case "count":
				r.PromptTokens = 0
			case "model":
				identity.Runtime.Revision = "changed"
			}
			if err = fresh.Apply(identity, &r, ex.RawResponse); err == nil {
				t.Fatal("tampered checkpoint accepted")
			}
			if fresh.Next().Ordinal != 0 {
				t.Fatal("tamper advanced cursor")
			}
		})
	}
	identity := synthesisIdentity()
	identity.Runtime.Revision = "changed"
	session := &synthesisTestSession{identity: identity}
	provider := &synthesisTestProvider{session: session}
	if _, err := c.Execute(context.Background(), c.Next(), identity, provider, func() error { return nil }); err == nil || provider.calls != 0 {
		t.Fatal("model changed between tree levels", err, provider.calls)
	}
}

func TestCompositionAllExcludedAndMutableCallerData(t *testing.T) {
	c, _, source, graphs, seeds := compositionFixture(t, 1)
	seeds[0].Candidate.Text = "Caller edited after preparing"
	prompt, err := c.Prompt(c.Next(), "en")
	if err != nil || strings.Contains(prompt.User, "Caller edited") {
		t.Fatal("mutable seed escaped", err)
	}
	seeds[0].Decision = "exclude"
	plan, err := BuildCompositionPlan("analysis", source, seeds)
	if err != nil {
		t.Fatal(err)
	}
	c, err = PrepareComposition(plan, source, graphs, seeds)
	if err != nil {
		t.Fatal(err)
	}
	if c.Next() != nil {
		t.Fatal("excluded candidates scheduled")
	}
	r, err := c.Report()
	if err != nil || len(r.Sections) != 0 || len(r.Gaps) != 1 || r.Gaps[0].Code != "composition_no_selected_candidates" || !r.SemanticReviewRequired {
		t.Fatal(r, err)
	}
}
