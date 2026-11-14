package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestResultHistoryRetainsTailAndRestrictsGraph(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), nil)
	defer r.Shutdown(context.Background())
	parent := &Task{ID: "root", Actor: "alice", ChildHistory: []ChildLink{{ID: "source", Spec: ChildSpec{Key: "source"}}, {ID: "reader", Spec: ChildSpec{Key: "reader", DependsOn: []string{"source"}}}, {ID: "sibling", Spec: ChildSpec{Key: "sibling"}}}}
	source := &Task{ID: "source", Actor: "alice", ParentID: parent.ID, Status: TaskCompleted, Result: strings.Repeat("文", 4000) + "\n[Final citation](https://example.org/evidence)"}
	reader := &Task{ID: "reader", Actor: "alice", ParentID: parent.ID}
	m.tasks[parent.ID], m.tasks[source.ID], m.tasks[reader.ID] = parent, source, reader
	ref := resultReference(source)
	for _, caller := range []*Task{parent, reader} {
		var output strings.Builder
		for offset := 0; ; {
			encoded, err := r.historyPage(context.Background(), caller, ref, offset)
			if err != nil {
				t.Fatal(err)
			}
			var page struct {
				Content  string `json:"content"`
				Next     int    `json:"next_offset"`
				Complete bool   `json:"complete"`
			}
			if err := json.Unmarshal([]byte(encoded), &page); err != nil {
				t.Fatal(err)
			}
			output.WriteString(page.Content)
			if page.Complete {
				break
			}
			if page.Next <= offset {
				t.Fatal("pagination did not advance")
			}
			offset = page.Next
		}
		if output.String() != source.Result {
			t.Fatal("result lost bytes or its final citation")
		}
	}
	for _, caller := range []*Task{{ID: "foreign", Actor: "alice"}, {ID: "sibling", Actor: "alice", ParentID: parent.ID}, {ID: "root", Actor: "bob"}} {
		if _, err := r.historyPage(context.Background(), caller, ref, 0); err == nil {
			t.Fatal("unlinked result was disclosed")
		}
	}
	for _, offset := range []int{-1, 1, len(source.Result) + 1} {
		if _, err := r.historyPage(context.Background(), parent, ref, offset); err == nil {
			t.Fatal("invalid byte offset accepted")
		}
	}
	source.Result += "changed"
	if _, err := r.historyPage(context.Background(), parent, ref, 0); err == nil {
		t.Fatal("changed dependency accepted old reference")
	}
	source.Result = strings.TrimSuffix(source.Result, "changed")
	source.Status = TaskRunning
	if _, err := r.historyPage(context.Background(), parent, ref, 0); err == nil {
		t.Fatal("unsettled dependency disclosed")
	}
}

func TestPartialDelegationSummaryPreservesUsefulOutput(t *testing.T) {
	results := []childOutcome{outcomeForChild("research", &Task{ID: "a", Status: TaskCompleted, Result: "Useful finding [1]"}), outcomeForChild("tests", &Task{ID: "b", Status: TaskFailed, Error: "runtime unavailable"})}
	summary := partialDelegationSummary(results)
	for _, want := range []string{"Useful finding [1]", "runtime unavailable", "not been independently verified", results[0].ResultReference} {
		if !strings.Contains(summary, want) {
			t.Fatalf("partial outcome lost %q", want)
		}
	}
}
