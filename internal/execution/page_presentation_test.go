package execution

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDirectoryPresentationSeparatesPageScopeAndPreservesReceipt(t *testing.T) {
	for _, offset := range []int{0, 2} {
		page := DirectoryPage{Location: Location{Host: "host", Path: "/approved"}, Offset: offset, TotalEntries: offset + 3, Entries: []Entry{{Name: "café 日本語.txt"}, {Name: "reports", Directory: true}, {Name: "line\nbreak"}}, Revision: strings.Repeat("a", 64), Complete: true}
		data, _ := json.Marshal(page)
		receipt := Receipt{State: "complete", Result: data}
		value, err := PresentReceipt(Reference{Host: "host", Session: "session"}, "file_list", receipt)
		var view struct {
			Text    string  `json:"text"`
			Receipt Receipt `json:"receipt"`
		}
		if err != nil || json.Unmarshal([]byte(value), &view) != nil {
			t.Fatal(value, err)
		}
		if !strings.Contains(view.Text, `"café 日本語.txt"`) || !strings.Contains(view.Text, `"reports/"`) || !strings.Contains(view.Text, `"line\nbreak"`) || !strings.Contains(view.Text, "No further page") {
			t.Fatal(view.Text)
		}
		if strings.Contains(view.Text, "earlier entries are not repeated") != (offset > 0) {
			t.Fatal("lost page scope", view.Text)
		}
		if string(view.Receipt.Result) != string(data) || string(receipt.Result) != string(data) {
			t.Fatal("canonical receipt rewritten")
		}
	}
}

func TestDirectoryPresentationDoesNotInventMissingEntries(t *testing.T) {
	value, err := PresentReceipt(Reference{Host: "host"}, "file_list", Receipt{State: "complete", Result: json.RawMessage(`{}`)})
	var view struct {
		Text *string `json:"text"`
	}
	if err != nil || json.Unmarshal([]byte(value), &view) != nil || view.Text != nil {
		t.Fatal(value, err)
	}
}
