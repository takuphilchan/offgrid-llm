package sessions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func TestSavedAttachmentCoverageCannotBeSilentlyChanged(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "modified", true: "removed"}[remove], func(t *testing.T) {
			dir := t.TempDir()
			scope := NewSessionManager(dir).WithAccess(Access{UserID: "alice"})
			if _, err := scope.Create("policy", "model"); err != nil {
				t.Fatal(err)
			}
			a, err := scope.CreateAttachment("policy", "policy.txt", "text/plain", []byte("Readable clause"))
			if err != nil {
				t.Fatal(err)
			}
			receipt := &document.Coverage{Status: "partial", TotalUnits: 2, ReadableUnits: 1, ReviewUnits: []int{2}}
			ready, err := scope.PrepareAttachmentEvidence("policy", a.ID, "Readable clause", "fixture-v1", "", "native_text_only", receipt)
			if err != nil {
				t.Fatal(err)
			}
			receipt.Status = "complete" // The caller must not mutate persisted evidence.
			scope = NewSessionManager(dir).WithAccess(Access{UserID: "alice"})
			read, _, err := scope.ReadAttachment("policy", a.ID)
			if err != nil || read.Evidence.Status != "partial" || read.Revision != ready.Revision {
				t.Fatal(read, err)
			}
			var file string
			if err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err == nil && strings.HasSuffix(p, a.ID+".json") {
					file = p
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var record attachmentRecord
			if err = json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if remove {
				record.Evidence = nil
			} else {
				record.Evidence.Status = "complete"
			}
			data, _ = json.Marshal(record)
			if err = os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err = scope.ReadAttachment("policy", a.ID); !errors.Is(err, ErrAttachmentUnavailable) {
				t.Fatalf("changed extraction coverage accepted: %v", err)
			}
		})
	}
}
