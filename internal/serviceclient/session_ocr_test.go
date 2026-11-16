package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionFileOCRSelectionRequiresCapabilityAndMatchingReceipt(t *testing.T) {
	for _, language := range []string{"chi_sim", "chi_sim+fra"} {
		t.Run(language, func(t *testing.T) { testSessionFileOCRSelection(t, language) })
	}
}

func testSessionFileOCRSelection(t *testing.T, selection string) {
	capability := "document-ocr-language-selection-v1"
	if strings.Contains(selection, "+") {
		capability = "document-ocr-mixed-languages-v1"
	}
	for _, mode := range []string{"supported", "old", "wrong"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			data := []byte("native text")
			sum := sha256.Sum256(data)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					caps := []string{}
					if strings.Contains(selection, "+") {
						caps = append(caps, "document-ocr-language-selection-v1")
					}
					if mode != "old" {
						caps = append(caps, capability)
					}
					json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
					return
				}
				posts++
				if r.URL.Query().Get("ocr_language") != selection {
					t.Error("selection lost")
				}
				language := selection
				if mode == "wrong" {
					language = "eng"
				}
				json.NewEncoder(w).Encode(map[string]any{"attachment": map[string]any{"id": "file-" + strings.Repeat("a", 32), "name": "source.txt", "bytes": len(data), "sha256": hex.EncodeToString(sum[:]), "status": "ready", "revision": strings.Repeat("b", 64), "ocr_language": language}})
			}))
			defer s.Close()
			c, err := New(s.URL, "", s.Client())
			if err != nil {
				t.Fatal(err)
			}
			a, err := c.UploadSessionFileWithOCRLanguage(context.Background(), "chat", "source.txt", data, selection)
			if mode == "supported" {
				if err != nil || a.OCRLanguage != selection {
					t.Fatal(a, err)
				}
			} else if err == nil {
				t.Fatal("unsupported/wrong selection accepted")
			}
			if mode == "old" && posts != 0 || mode != "old" && posts != 1 {
				t.Fatal("unexpected upload/retry", posts)
			}
		})
	}
}
