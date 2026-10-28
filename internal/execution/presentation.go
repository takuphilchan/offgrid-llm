package execution

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// PresentReceipt bounds the model/task projection without rewriting raw host
// journal evidence. More output is retrieved by handle, never by restarting.
func PresentReceipt(ref Reference, operation string, original Receipt) (string, error) {
	receipt := original
	var text *string
	if operation == "file_write" && receipt.State == "complete" {
		var written WriteResult
		if json.Unmarshal(receipt.Result, &written) == nil && written.Host == ref.Host && written.Verified && len(written.Revision) == 64 {
			value := "The proposed bytes were written and independently read back exactly. This write is complete; do not write them again merely to confirm it. Check any remaining user requirements using the recorded evidence or a read. Mechanical equality does not prove the proposed content satisfied every requirement."
			text = &value
		}
	}
	if operation == "file_list" && receipt.State == "complete" {
		var page DirectoryPage
		if json.Unmarshal(receipt.Result, &page) == nil && page.Entries != nil && len(page.Entries) <= MaxEntriesPerPage {
			names := make([]string, 0, len(page.Entries))
			for _, entry := range page.Entries {
				name := entry.Name
				if entry.Directory {
					name += "/"
				}
				names = append(names, name)
			}
			encoded, _ := json.Marshal(names)
			if len(encoded) <= MaxPageBytes {
				value := "Observed entries in this page: " + string(encoded) + "."
				if page.NextOffset != nil {
					value += fmt.Sprintf(" Continue only at next_offset %d with this directory's revision.", *page.NextOffset)
				} else if page.Complete {
					value += " No further page remains. Use these observed names when answering a listing request; do not invent another offset."
				} else {
					value += " Completeness is unknown; no usable continuation was recorded."
				}
				if page.Offset > 0 {
					value += fmt.Sprintf(" This page starts at entry %d; earlier entries are not repeated here.", page.Offset)
				}
				text = &value
			}
		}
	}
	if operation == "file_read" && receipt.State == "complete" {
		var page FilePage
		if json.Unmarshal(receipt.Result, &page) == nil && utf8.Valid(page.Data) {
			value := string(page.Data)
			text = &value
		}
	}
	if strings.HasPrefix(operation, "process_") {
		var process ProcessResult
		if json.Unmarshal(receipt.Result, &process) == nil && process.Handle != "" {
			if len(process.Output) > MaxPageBytes {
				total := len(process.Output)
				process.Output = process.Output[:MaxPageBytes]
				next := MaxPageBytes
				page := ProcessPage{ProcessResult: process, Offset: 0, NextOffset: &next, CapturedBytes: total, Complete: false}
				receipt.Result, _ = json.Marshal(page)
			}
			if utf8.Valid(process.Output) {
				value := string(process.Output)
				text = &value
			}
		}
	}
	encoded, err := json.Marshal(map[string]any{"host": ref.Host, "session": ref.Session, "operation": operation, "receipt": receipt, "text": text, "untrusted": true})
	return string(encoded), err
}

func MutatingOperation(operation string) bool {
	return operation == "file_write" || operation == "directory_change" || operation == "process_start"
}
