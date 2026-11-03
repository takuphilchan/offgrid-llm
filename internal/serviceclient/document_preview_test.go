package serviceclient

import (
	"context"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestPreviewRejectsMalformedDigestBeforeTransport(t *testing.T) {
	client, err := New("http://127.0.0.1:1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PreviewSessionFile(context.Background(), "paper", sessions.Attachment{
		ID: "file-" + strings.Repeat("a", 32), SHA256: strings.Repeat("z", 64),
	})
	if err == nil || err.Error() != "invalid attachment identity" {
		t.Fatal("invalid source reached transport", err)
	}
}
