package execution

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProcessDescendantProofWireContract(t *testing.T) {
	zero := 0
	for _, tc := range []struct {
		name  string
		proof *ProcessDescendants
		state string
		valid bool
	}{
		{"absent", nil, "unavailable", false},
		{"unknown", &ProcessDescendants{Version: 1, State: "unknown"}, "unknown", false},
		{"future", &ProcessDescendants{Version: 2, State: "settled"}, "invalid", false},
		{"invalid", &ProcessDescendants{Version: 1, State: "complete"}, "invalid", false},
		{"pending-terminal", &ProcessDescendants{Version: 1, State: "pending"}, "invalid", false},
		{"settled", &ProcessDescendants{Version: 1, State: "settled"}, "settled", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := ProcessResult{Handle: "observed", State: "exited", ExitCode: &zero, Descendants: tc.proof}
			raw, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var got ProcessResult
			if err = json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if ProcessDescendantsSettled(got) != tc.valid || ProcessDescendantState(got) != tc.state {
				t.Fatal(string(raw), got)
			}
			if tc.proof == nil && strings.Contains(string(raw), "descendants") {
				t.Fatal("legacy observation synthesized proof")
			}
			outcome := processOutcomeSummary("complete", got)
			if tc.valid {
				if !strings.Contains(outcome, "completed successfully") {
					t.Fatal(outcome)
				}
			} else if !strings.Contains(outcome, "leader exited") || !strings.Contains(outcome, "unverified") {
				t.Fatal("leader outcome hid missing descendant proof", outcome)
			}
		})
	}
	running := ProcessResult{State: "running", Descendants: &ProcessDescendants{Version: 1, State: "settled"}}
	if ProcessDescendantsSettled(running) || ProcessDescendantState(running) != "invalid" {
		t.Fatal("running process claimed settlement")
	}
}

func TestProcessLeaderSnapshotDoesNotSettleUnobservedDescendants(t *testing.T) {
	zero := 0
	process := &ownedProcess{handle: "observed", state: "exited", exit: &zero, started: time.Now().Add(-time.Second), elapsed: time.Second}
	result := process.snapshot()
	if result.State != "exited" || result.ExitCode == nil || *result.ExitCode != 0 || ProcessDescendantState(result) != "unknown" || result.Runtime == nil || result.Runtime.Settled {
		t.Fatal(result)
	}
}
