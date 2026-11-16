package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestDocumentThumbnailHTTPIdentityAndCurrentAccess(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewGray(image.Rect(0, 0, 800, 1000))); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data.Bytes())
	sha := hex.EncodeToString(sum[:])
	for _, tc := range []struct {
		query   string
		allowed bool
		status  int
	}{
		{"?page=1&thumbnail=true&sha256=" + sha, true, 200},
		{"?page=1&thumbnail=true", false, 404},
		{"?page=1&thumbnail=true&sha256=" + strings.Repeat("0", 64), true, 409},
		{"?thumbnail=true", true, 400},
		{"?page=1&thumbnail=other", true, 400},
		{"?page=1&thumbnail=true&thumbnail=false", true, 400},
	} {
		w := httptest.NewRecorder()
		serveDocumentPreview(w, httptest.NewRequest("GET", "/preview"+tc.query, nil), rag.NewDocumentParser(), "scan.png", data.Bytes(), func() bool { return tc.allowed })
		if w.Code != tc.status {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.status == 200 {
			cfg, err := png.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
			if err != nil || cfg.Width > 192 || cfg.Height > 256 || w.Header().Get("X-OffGrid-Source-SHA256") != sha || w.Header().Get("X-OffGrid-Preview") != "thumbnail-v1" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(cfg, err, w.Header())
			}
		} else if w.Header().Get("Content-Type") == "image/png" {
			t.Fatal("denied source delivered")
		}
	}
}
