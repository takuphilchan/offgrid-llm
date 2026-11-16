package sessions

import "testing"

func TestAttachmentReviewPreservesAuthorityAndTextIntegrity(t *testing.T) {
	scope, directory, a, graph := preparedStructuredAttachment(t)
	if _, text, _, err := scope.AttachmentReviewText("policy", a.ID); err != nil || text != graph.Text {
		t.Fatal(text, err)
	}
	other := NewSessionManager(directory).WithAccess(Access{UserID: "bob"})
	if _, _, _, err := other.AttachmentReviewText("policy", a.ID); err == nil {
		t.Fatal("private review leaked")
	}
	root, dir, err := scope.attachmentRoot("policy", false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	record, err := readAttachmentRecord(root, dir, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Text = "substituted source"
	if err = writeAttachmentRecord(root, dir, record); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = scope.AttachmentReviewText("policy", a.ID); err == nil {
		t.Fatal("substituted text accepted")
	}
}
