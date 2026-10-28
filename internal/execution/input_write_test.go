package execution

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputSelectionPreservesExactBytesAndRejectsAmbiguity(t *testing.T) {
	for _, content := range []string{"café 日本語\r\n1| genuine text\n", "", strings.Repeat("x", MaxInlineWriteBytes)} {
		input := "Preserve <content>" + content + "</content> exactly"
		a := InputWriteArguments{Path: "result.txt", Input: "request", After: "<content>", Before: "</content>"}
		payload, proof, err := SelectInput(input, a)
		if err != nil || string(payload.Data) != content || proof.ContentDigest != digest([]byte(content)) || proof.InputDigest != digest([]byte(input)) || input[proof.StartByte:proof.EndByte] != content {
			t.Fatal(proof, err)
		}
	}
	for _, input := range []string{"missing", "</content>wrong<content>", "<content>one<content>two</content>", "<content>one</content>two</content>", "<content>" + strings.Repeat("x", MaxInlineWriteBytes+1) + "</content>", "<content>\xff</content>"} {
		if _, _, err := SelectInput(input, InputWriteArguments{Path: "result.txt", Input: "request", After: "<content>", Before: "</content>"}); err == nil {
			t.Fatal("ambiguous/invalid input accepted")
		}
	}
}

func TestInputWriteStrictArguments(t *testing.T) {
	for _, args := range []string{`{"path":"x","input":"request","after":"<a>","before":"</a>","actor":"foreign"}`, `{"path":"x","input":"request","input":"foreign","after":"a","before":"b"}`, `{"path":"x","input":null,"after":"a","before":"b"}`, `{"path":"x","input":"request","after":"","before":"b"}`, `{"path":"x","input":"request","after":"a","before":"b","revision":"invented"}`, `{"path":"x","input":"request","after":"\ud800","before":"b"}`} {
		if _, err := ParseInputWrite([]byte(args)); err == nil {
			t.Fatal("invalid selection accepted", args)
		}
	}
}

func TestInputWriteRequiresMatchingIndependentReceipt(t *testing.T) {
	payload, proof, err := SelectInput("<a>café 日本語\r\n</a>", InputWriteArguments{Path: "result.txt", Input: "request", After: "<a>", Before: "</a>"})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(payload)
	request := Request{Operation: "file_write", Arguments: args, Binding: Binding{Host: "host"}}
	written := WriteResult{Location: Location{Host: "host", Path: "/approved/result.txt"}, Revision: proof.ContentDigest, Bytes: len(payload.Data), Verified: true, Created: true}
	receipt := func(w WriteResult) Receipt {
		data, _ := json.Marshal(w)
		return Receipt{State: "complete", Result: data}
	}
	original := receipt(written)
	view, err := PresentInputWrite(Reference{Host: "host", Session: "session"}, request, original, proof)
	if err != nil || !strings.Contains(view, `"source_input"`) || string(original.Result) != string(receipt(written).Result) {
		t.Fatal(view, err)
	}
	for _, change := range []func(*WriteResult){func(w *WriteResult) { w.Verified = false }, func(w *WriteResult) { w.Bytes++ }, func(w *WriteResult) { w.Revision = strings.Repeat("0", 64) }, func(w *WriteResult) { w.Host = "foreign" }} {
		bad := written
		change(&bad)
		if _, err := PresentInputWrite(Reference{}, request, receipt(bad), proof); err == nil {
			t.Fatal("unverified source write accepted", bad)
		}
	}
	for _, state := range []string{"partial", "uncertain", "rejected", "dispatched"} {
		bad := original
		bad.State = state
		if WriteReceiptMatches(request, bad) {
			t.Fatal("nonterminal receipt accepted", state)
		}
	}
	if Permitted(InputWriteTool, Permissions{Read: true}) || !Permitted(InputWriteTool, Permissions{Write: true}) {
		t.Fatal("source writing did not require write authority")
	}
}
