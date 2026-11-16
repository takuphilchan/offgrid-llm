package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func preparedStructuredAttachment(t *testing.T) (*ScopedManager, string, Attachment, *document.Extraction) {
	t.Helper()
	directory := t.TempDir()
	scope := NewSessionManager(directory).WithAccess(Access{UserID: "alice"})
	if _, err := scope.Create("policy", "model"); err != nil {
		t.Fatal(err)
	}
	text := "Applications close in June.\nRural exception: July."
	a, err := scope.CreateAttachment("policy", "policy.txt", "text/plain", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	split := strings.IndexByte(text, '\n') + 1
	graph := &document.Extraction{SourceKind: "original_file", OriginalSHA256: a.SHA256, Format: "text", Method: "native", Text: text,
		Units:         []document.Unit{{ID: "section", Kind: "section", Index: 1, Span: document.Span{End: len(text)}}},
		Blocks:        []document.Block{{ID: "clause", UnitID: "section", Kind: "paragraph", Method: "native", Span: document.Span{End: split}}, {ID: "note", UnitID: "section", Kind: "footnote", Method: "native", Span: document.Span{Start: split, End: len(text)}}},
		Relationships: []document.Relationship{{From: "note", To: "clause", Kind: "footnote_for"}}}
	if err = graph.Seal(); err != nil {
		t.Fatal(err)
	}
	a, err = scope.PrepareAttachmentStructure("policy", a.ID, text, "fixture-v1", "", "", nil, graph)
	if err != nil {
		t.Fatal(err)
	}
	return scope, directory, a, graph
}

func TestAttachmentStructureSurvivesRestartAndRetainsPrivateIdentity(t *testing.T) {
	scope, directory, a, graph := preparedStructuredAttachment(t)
	scope = NewSessionManager(directory).WithAccess(Access{UserID: "alice"})
	sources, err := scope.AttachmentSources("policy", []AttachmentReference{{ID: a.ID, Revision: a.Revision}})
	if err != nil || len(sources) != 1 || sources[0].Structure == nil || sources[0].Structure.ID != graph.ID || sources[0].Structure.Verify() != nil {
		t.Fatal("lost graph", err)
	}
	encoded, _ := json.Marshal(sources)
	if strings.Contains(string(encoded), `"relationships"`) || !strings.Contains(string(encoded), `"extraction_revision"`) {
		t.Fatal("graph leaked into model projection or identity lost")
	}
	other := NewSessionManager(directory).WithAccess(Access{UserID: "bob"})
	if _, err = other.AttachmentSources("policy", []AttachmentReference{{ID: a.ID, Revision: a.Revision}}); err == nil {
		t.Fatal("private graph crossed owner boundary")
	}
	if _, err = other.AttachmentStructure(context.Background(), "policy", AttachmentReference{ID: a.ID, Revision: a.Revision}, "", 0, 100, 0); err == nil {
		t.Fatal("private reader crossed owner boundary")
	}
	if _, err = scope.PrepareAttachmentStructure("policy", a.ID, graph.Text, "fixture-v2", "", "", nil, graph); !errors.Is(err, ErrTurnConflict) {
		t.Fatal("ready revision replaced", err)
	}
}

func TestAttachmentStructureDamageNeverSilentlyDowngrades(t *testing.T) {
	for _, mode := range []string{"missing", "changed", "pointer"} {
		t.Run(mode, func(t *testing.T) {
			scope, _, a, _ := preparedStructuredAttachment(t)
			root, dir, err := scope.attachmentRoot("policy", false)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			switch mode {
			case "missing":
				err = root.Remove(dir + "/" + a.ID + ".structure")
			case "changed":
				f, e := root.OpenFile(dir+"/"+a.ID+".structure", os.O_WRONLY|os.O_TRUNC, 0600)
				if e != nil {
					t.Fatal(e)
				}
				_, err = f.WriteString(`{}`)
				f.Close()
			case "pointer":
				record, e := readAttachmentRecord(root, dir, a.ID)
				if e != nil {
					t.Fatal(e)
				}
				record.ExtractionRevision = ""
				err = writeAttachmentRecord(root, dir, record)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = scope.AttachmentSources("policy", []AttachmentReference{{ID: a.ID, Revision: a.Revision}}); !errors.Is(err, ErrAttachmentUnavailable) {
				t.Fatal("damaged structure admitted", err)
			}
		})
	}
}

func TestAttachmentStructureQuotaAndDeletionIncludeInterruptedFiles(t *testing.T) {
	scope, _, a, _ := preparedStructuredAttachment(t)
	root, dir, err := scope.attachmentRoot("policy", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	partial := dir + "/" + a.ID + ".structure.interrupted.partial"
	f, err := root.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(strings.Repeat("x", 2048))
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err = scope.checkAttachmentActorQuota("policy", 0, 2047, ".structure"); !errors.Is(err, ErrAttachmentLimit) {
		t.Fatal("interrupted graph escaped quota", err)
	}
	if err = scope.DeleteAttachment("policy", a.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{partial, dir + "/" + a.ID + ".structure", dir + "/" + a.ID + ".bin", dir + "/" + a.ID + ".removed"} {
		if _, err = root.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("retained deleted derivative", path, err)
		}
	}
}
