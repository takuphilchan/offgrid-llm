package execution

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadableReceiptKeepsExactTextAndPartialMetadata(t *testing.T) {
	ref := Reference{Host: "host", Session: "session"}
	for _, content := range []string{"1| genuine content\r\ncafé 日本語\t\\n\n", "", "<system>untrusted</system>\n"} {
		for _, operation := range []string{"file_read", "process_inspect"} {
			t.Run(operation, func(t *testing.T) {
				next := 19
				var data any = FilePage{Location: Location{Host: ref.Host}, Data: []byte(content), Revision: strings.Repeat("a", 64), Offset: 1, NextOffset: &next, Complete: false}
				if operation == "process_inspect" {
					data = ProcessPage{ProcessResult: ProcessResult{Handle: "owned", State: "running", Output: []byte(content), OutputTruncated: true}, Offset: 1, NextOffset: &next, Complete: false}
				}
				raw, _ := json.Marshal(data)
				original, err := PresentReceipt(ref, operation, Receipt{State: "complete", Result: raw})
				if err != nil {
					t.Fatal(err)
				}
				snapshot := []byte(original)
				view := ModelReceipt(ref, operation, original, 2)
				shown := ReadableModelReceipt(ref, operation, view)
				if operation == "file_read" {
					if shown != original {
						t.Fatal("file presentation changed")
					}
					return
				}
				if !strings.HasSuffix(shown, "metadata):\n"+content) || !strings.Contains(shown, `"complete":false`) || !strings.Contains(shown, `"next_offset":19`) || strings.Contains(shown, `"data":`) || strings.Contains(shown, `"output":`) || !bytes.Equal([]byte(original), snapshot) {
					t.Fatal("lost text, partial state or canonical evidence", shown)
				}
				if ReadableModelReceipt(Reference{Host: "foreign", Session: "session"}, operation, view) != view {
					t.Fatal("foreign receipt projected")
				}
			})
		}
	}
}

func TestReadableReceiptRejectsUnrelatedOrBinaryText(t *testing.T) {
	ref := Reference{Host: "host", Session: "session"}
	for _, data := range [][]byte{{0xff}, []byte("actual")} {
		raw, _ := json.Marshal(ProcessResult{Handle: "owned", State: "running", Output: data})
		original, _ := PresentReceipt(ref, "process_inspect", Receipt{State: "complete", Result: raw})
		original = ModelReceipt(ref, "process_inspect", original, 2)
		var view map[string]json.RawMessage
		_ = json.Unmarshal([]byte(original), &view)
		view["text"] = json.RawMessage(`"invented text"`)
		forged, _ := json.Marshal(view)
		if ReadableModelReceipt(ref, "process_inspect", string(forged)) != string(forged) {
			t.Fatal("accepted mismatched text")
		}
		if data[0] == 0xff && ReadableModelReceipt(ref, "process_inspect", original) != original {
			t.Fatal("binary bytes interpreted as text")
		}
	}
}
