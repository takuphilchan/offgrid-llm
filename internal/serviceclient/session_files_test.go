package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestSessionFileUploadPreservesBytesAndAuthentication(t *testing.T) {
	data := []byte("café 日本語\n1| genuine source line\n")
	sum := sha256.Sum256(data)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer private" || r.URL.Path != "/v1/sessions/my chat/attachments" {
			t.Error("wrong authenticated target")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		got, _ := io.ReadAll(f)
		if string(got) != string(data) || header.Filename != "notes.txt" {
			t.Error("source bytes or filename changed")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"attachment": sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: header.Filename, Bytes: int64(len(got)), SHA256: hex.EncodeToString(sum[:]), Revision: strings.Repeat("c", 64), Status: "ready"}})
	}))
	defer server.Close()
	client, _ := New(server.URL, "private", nil)
	a, err := client.UploadSessionFile(context.Background(), "my chat", "notes.txt", data)
	if err != nil || a.Status != "ready" || calls != 1 {
		t.Fatalf("upload: %+v %v calls=%d", a, err, calls)
	}
	for _, name := range []string{"../notes.txt", "note\n.txt", ""} {
		if _, err := client.UploadSessionFile(context.Background(), "my chat", name, data); err == nil {
			t.Fatal("unsafe filename accepted")
		}
	}
	if calls != 1 {
		t.Fatal("invalid input dispatched")
	}
}

func TestSessionFileFailureKeepsOnlySafeIdentity(t *testing.T) {
	id := "file-" + strings.Repeat("a", 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_ = json.NewEncoder(w).Encode(map[string]any{"attachment": map[string]string{"id": id}, "error": map[string]string{"code": "document_ocr_required", "message": "secret\x1b[2J"}})
	}))
	defer server.Close()
	client, _ := New(server.URL, "private", nil)
	_, err := client.UploadSessionFile(context.Background(), "chat", "scan.pdf", []byte("data"))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "document_ocr_required" || failure.AttachmentID != id || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe or lost recovery: %+v", err)
	}
}

func TestSessionFileResponseCannotInventSourceIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"attachment": sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: "notes.txt", Bytes: 4, SHA256: strings.Repeat("0", 64), Revision: strings.Repeat("c", 64), Status: "ready"}})
	}))
	defer server.Close()
	client, _ := New(server.URL, "", nil)
	if _, err := client.UploadSessionFile(context.Background(), "chat", "notes.txt", []byte("data")); err == nil {
		t.Fatal("mismatched digest accepted")
	}
}
