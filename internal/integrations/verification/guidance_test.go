package verification

import (
	"strings"
	"testing"
)

func TestRejectionGuidanceIsSpecificAndBounded(t *testing.T) {
	mismatch := rejectionGuidance("content_mismatch")
	if !strings.Contains(mismatch, "differs from the source bytes") || strings.Contains(mismatch, "absolute local") || !strings.Contains(mismatch, "genuine numeric prefixes") {
		t.Fatal(mismatch)
	}
	for _, code := range []string{"content_mismatch", "complete_source_read_required", "read_target_unavailable", "operation_invalid", "write_outside_expected_destination", "write_attempt_in_no_changes_run", "unknown-private-text"} {
		message := rejectionGuidance(code)
		if len(message) == 0 || len(message) > 1024 || strings.Contains(message, "unknown-private-text") {
			t.Fatal(message)
		}
	}
}
