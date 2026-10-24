package verification

// Explain the deterministic rejection, not a guessed path problem. This never
// supplies replacement content or changes the original tool arguments/results.
func rejectionGuidance(code string) string {
	switch code {
	case "content_mismatch":
		return "The proposed write differs from the source bytes; it was not executed. Copy actual file contents, not the tool wrapper or display-only line numbers. Preserve genuine numeric prefixes, Unicode and whitespace. Do not claim an exact match without successful read-back."
	case "complete_source_read_required":
		return "A complete source read is required before writing. Read the source from its beginning without a partial-range limit; do not reconstruct missing contents."
	case "read_target_unavailable":
		return "The requested file does not exist as a readable file in the checked workspace. A rejected write did not create it. Review the preceding result; do not claim completion."
	case "operation_invalid":
		return "Use an absolute local file path inside the declared workspace and valid read/write arguments. No rejected action was executed."
	case "write_outside_expected_destination":
		return "This contract permits writing only its declared destination. The proposed write was not executed."
	case "write_attempt_in_no_changes_run":
		return "This contract requires no workspace changes. Writing is not permitted; report the rejection without claiming verification passed."
	default:
		return "The file operation could not be verified. Inspect the contract and previous results; do not retry an uncertain mutation or claim verification succeeded."
	}
}
