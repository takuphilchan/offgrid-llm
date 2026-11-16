package serviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestPreviewImageFrameRequiresBoundedEncodedDimensions(t *testing.T) {
	attachment := sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), SHA256: strings.Repeat("b", 64), Name: "scan.tiff"}
	for _, tc := range []struct {
		name  string
		frame any
		valid bool
	}{
		{"legacy", nil, true},
		{"rotated", map[string]int{"width": 100, "height": 200, "orientation": 6}, true},
		{"empty", map[string]int{"width": 0, "height": 200, "orientation": 1}, false},
		{"oversized", map[string]int{"width": 8193, "height": 200, "orientation": 1}, false},
		{"allocation", map[string]int{"width": 8192, "height": 8192, "orientation": 1}, false},
		{"unknown orientation", map[string]int{"width": 100, "height": 200, "orientation": 9}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("sha256") != attachment.SHA256 {
					t.Error("missing source pin")
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"name": attachment.Name, "sha256": attachment.SHA256, "units": []any{map[string]any{"kind": "page", "index": 1, "source_sha256": attachment.SHA256, "image_geometry": tc.frame}}})
			}))
			defer h.Close()
			c, err := New(h.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.PreviewSessionFile(context.Background(), "paper", attachment)
			if (err == nil) != tc.valid {
				t.Fatal(tc.name, err)
			}
		})
	}
}
