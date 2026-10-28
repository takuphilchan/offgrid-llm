package execution

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// ModelReceipt presents observed process text once. The canonical step and host
// journal keep the original bytes; no assistant output or invocation is changed.
func ModelReceipt(ref Reference, operation, original string, step int) string {
	if operation != "process_start" && operation != "process_inspect" && operation != "process_wait" && operation != "process_stop" || step < 1 || len(original) > 256<<10 || jsonwire.Validate([]byte(original)) != nil {
		return original
	}
	var view map[string]json.RawMessage
	var receipt map[string]json.RawMessage
	var result map[string]json.RawMessage
	var binding struct{ Host, Session, Operation string }
	if json.Unmarshal([]byte(original), &binding) != nil || binding.Host != ref.Host || binding.Session != ref.Session || binding.Operation != operation || json.Unmarshal([]byte(original), &view) != nil || json.Unmarshal(view["receipt"], &receipt) != nil || json.Unmarshal(receipt["result"], &result) != nil {
		return original
	}
	var process ProcessResult
	if json.Unmarshal(receipt["result"], &process) != nil || process.Handle == "" || len(process.Output) > MaxPageBytes {
		return original
	}
	if utf8.Valid(process.Output) {
		delete(result, "output")
		view["text"], _ = json.Marshal(string(process.Output))
		view["output_encoding"], _ = json.Marshal("utf-8")
	} else {
		view["text"] = json.RawMessage("null")
		view["output_encoding"], _ = json.Marshal("base64; output is not complete UTF-8 text")
	}
	if process.State == "running" {
		view["progress"], _ = json.Marshal("The command is still running. A complete wait/inspect receipt means the observation finished, NOT the command. Wait/inspect this same handle until it has an exit code; do not restart it or report completion.")
	} else if process.ExitCode != nil {
		view["progress"], _ = json.Marshal("The command ended. Report its actual exit code and observed output; do not restart it merely to obtain a response.")
	}
	var receiptState string
	if json.Unmarshal(receipt["state"], &receiptState) == nil {
		if outcome := processOutcomeSummary(receiptState, process); outcome != "" {
			view["observed_outcome"], _ = json.Marshal(outcome)
		}
	}
	view["original_output_reference"], _ = json.Marshal(fmt.Sprintf("step:%d", step))
	view["output_sha256"], _ = json.Marshal(digest(process.Output))
	receipt["result"], _ = json.Marshal(result)
	view["receipt"], _ = json.Marshal(receipt)
	encoded, err := json.Marshal(view)
	if err != nil {
		return original
	}
	return string(encoded)
}

// Interpret only a complete, consistent terminal receipt. Exit zero describes
// the command's status, not correctness of its output or a larger task.
func processOutcomeSummary(receiptState string, p ProcessResult) string {
	if receiptState != "complete" || p.ExitCode == nil {
		return ""
	}
	if p.State == "exited" && *p.ExitCode == 0 {
		return "This command completed successfully with exit code 0. Earlier rejected proposals were not dispatched and do not change this command's observed outcome. This does not establish the correctness of its output or completion of other work."
	}
	if (p.State == "failed" || p.State == "exited") && *p.ExitCode != 0 {
		return fmt.Sprintf("This command ended with nonzero exit code %d. Report the unsuccessful command outcome and observed output; do not claim it passed.", *p.ExitCode)
	}
	if p.State == "cancelled" || p.State == "timed_out" {
		return "This command was cancelled or timed out. Do not treat its termination as successful completion."
	}
	return ""
}
