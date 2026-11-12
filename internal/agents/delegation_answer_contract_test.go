package agents

import (
	"strings"
	"testing"
)

func TestDelegatedAnswerChecksExplicitLengthAndRates(t *testing.T) {
	task := &Task{
		Prompt:       "Have Source A check 96 of 120 tasks completed, and 72 of those 96 passed review. Have Source B check 94 of 120 tasks completed, and 72 of those 94 passed review. Write a 150-word briefing showing each completion and pass rate.",
		ChildHistory: []ChildLink{{}},
	}
	missing := delegatedAnswerFeedback(task, "Source A: 96 of 120. Source B: 94 of 120. The counts disagree.")
	if !strings.Contains(missing, "150-word") || !strings.Contains(missing, "80.0%") || !strings.Contains(missing, "75.0%") || !strings.Contains(missing, "78.3%") || !strings.Contains(missing, "76.6%") {
		t.Fatalf("mechanical omissions went undetected: %s", missing)
	}
	answer := "Source A completed 96 of 120 tasks (80%) and passed 72 of 96 reviews (75%). Source B completed 94 of 120 tasks (78.3%) and passed 72 of 94 reviews (76.6%). " + strings.Repeat("Both reports agree on the reviewed count but differ on completed tasks. ", 9)
	if missing = delegatedAnswerFeedback(task, answer); missing != "" {
		t.Fatalf("valid rates and approximately 150 words rejected: %s", missing)
	}
	task.ChildHistory = nil
	if missing = delegatedAnswerFeedback(task, "short"); missing != "" {
		t.Fatalf("ordinary answer received delegation contract: %s", missing)
	}
	task.ChildHistory = []ChildLink{{}}
	task.Prompt = "Have the researcher gather five findings and the writer prepare a 300-word briefing."
	if missing = delegatedAnswerFeedback(task, "A fixture briefing with no supplied numerical rates."); missing != "" {
		t.Fatalf("unrelated briefing received the supplied-figures contract: %s", missing)
	}
}
