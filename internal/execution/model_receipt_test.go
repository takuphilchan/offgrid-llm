package execution

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestModelProcessTextPreservesCanonicalBytesAndTerminalFacts(t *testing.T) {
	ref := Reference{Host: "host", Session: "session"}
	exit := 7
	for _, output := range [][]byte{[]byte("FAILED — café 日本語\n"), {0xff, 0xfe}, {}} {
		process := ProcessPage{ProcessResult: ProcessResult{Handle: "owned", State: "failed", ExitCode: &exit, Output: output, OutputTruncated: true}, Offset: 2, CapturedBytes: 100, Complete: false}
		raw, _ := json.Marshal(process)
		receipt := Receipt{State: "complete", Result: raw}
		original, err := PresentReceipt(ref, "process_inspect", receipt)
		if err != nil {
			t.Fatal(err)
		}
		before := append([]byte(nil), raw...)
		shown := ModelReceipt(ref, "process_inspect", original, 3)
		var view struct {
			Text      *string `json:"text"`
			Reference string  `json:"original_output_reference"`
			Digest    string  `json:"output_sha256"`
			Receipt   struct {
				Result ProcessPage `json:"result"`
			} `json:"receipt"`
		}
		if json.Unmarshal([]byte(shown), &view) != nil || view.Reference != "step:3" || view.Digest != digest(output) || !bytes.Equal(receipt.Result, before) || view.Receipt.Result.ExitCode == nil || *view.Receipt.Result.ExitCode != 7 || !view.Receipt.Result.OutputTruncated || view.Receipt.Result.Offset != 2 || view.Receipt.Result.Complete {
			t.Fatal("projection changed raw evidence or completeness", shown)
		}
		if len(output) > 0 && output[0] == 0xff {
			if view.Text != nil || !bytes.Equal(view.Receipt.Result.Output, output) {
				t.Fatal("binary output was rewritten", shown)
			}
		} else if view.Text == nil || *view.Text != string(output) || strings.Contains(shown, `"output":`) {
			t.Fatal("text was lost or duplicated as base64", shown)
		}
		if ModelReceipt(Reference{Host: "foreign", Session: "session"}, "process_inspect", original, 3) != original {
			t.Fatal("foreign observation projected")
		}
	}
}
