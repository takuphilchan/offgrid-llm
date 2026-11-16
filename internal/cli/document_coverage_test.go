package cli

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestDocumentCoverageNoticePrioritizesMissingExceptionAcrossSources(t *testing.T) {
	saved := &sessions.Session{Messages: []sessions.Message{{Role: "assistant", AttachmentSelections: []sessions.AttachmentSelection{{Mode: "excerpts"}, {Mode: "excerpts", LinkedEvidence: &document.SelectionReceipt{LinksComplete: false}}}}}}
	if notice := DocumentCoverageNotice(saved); !strings.Contains(notice, "exceptions were omitted") {
		t.Fatal(notice)
	}
	saved.Messages[0].AttachmentSelections[1].LinkedEvidence.LinksComplete = true
	if notice := DocumentCoverageNotice(saved); !strings.Contains(notice, "selected document passages") {
		t.Fatal(notice)
	}
}
