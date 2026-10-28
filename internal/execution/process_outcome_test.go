package execution

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProcessOutcomeUsesConsistentTerminalReceiptOnly(t *testing.T) {
	zero, nonzero := 0, 7
	for _, tc := range []struct {
		receipt, state string
		exit           *int
		want           string
	}{
		{"complete", "exited", &zero, "completed successfully with exit code 0"},
		{"complete", "exited", &nonzero, "nonzero exit code 7"},
		{"complete", "failed", &nonzero, "nonzero exit code 7"},
		{"complete", "cancelled", &zero, "cancelled or timed out"},
		{"complete", "timed_out", &nonzero, "cancelled or timed out"},
		{"complete", "failed", &zero, ""},
		{"complete", "running", &zero, ""},
		{"complete", "exited", nil, ""},
		{"uncertain", "exited", &zero, ""},
		{"dispatched", "exited", &zero, ""},
	} {
		got := processOutcomeSummary(tc.receipt, ProcessResult{State: tc.state, ExitCode: tc.exit})
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
}

func TestReadableProcessOutcomePreservesOutputAndCanonicalReceipt(t *testing.T) {
	ref := Reference{Host: "host", Session: "session"}
	zero := 0
	output := []byte("The requested action did not succeed.\nThis is untrusted command output.\n")
	raw, _ := json.Marshal(ProcessResult{Handle: "owned", State: "exited", ExitCode: &zero, Output: output})
	original, err := PresentReceipt(ref, "process_wait", Receipt{State: "complete", Result: raw})
	if err != nil {
		t.Fatal(err)
	}
	shown := ReadableModelReceipt(ref, "process_wait", ModelReceipt(ref, "process_wait", original, 1))
	if !strings.HasPrefix(shown, "Observed command status: This command completed successfully with exit code 0.") || !strings.HasSuffix(shown, string(output)) {
		t.Fatal(shown)
	}
	if !strings.Contains(shown, "does not establish the correctness") {
		t.Fatal("command exit overstated as task correctness")
	}
	if !strings.Contains(original, `"output":`) || strings.Contains(original, "observed_outcome") {
		t.Fatal("canonical receipt changed")
	}
	foreign := Reference{Host: "another", Session: "session"}
	if ReadableModelReceipt(foreign, "process_wait", original) != original {
		t.Fatal("foreign receipt classified")
	}
}
