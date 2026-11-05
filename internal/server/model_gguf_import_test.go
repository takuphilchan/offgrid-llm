package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func offlineGGUFRequest(t *testing.T) *http.Request {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("GGUF")
	write := func(v any) {
		t.Helper()
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	text := func(v string) { write(uint64(len(v))); b.WriteString(v) }
	write(uint32(3))
	write(uint64(1))
	write(uint64(1))
	text("general.architecture")
	write(uint32(8))
	text("qwen2")
	data := append(b.Bytes(), make([]byte, (1<<20)-b.Len())...)
	r := httptest.NewRequest("POST", modelGGUFImportPath+"?filename=fixture.gguf&size=1048576", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/octet-stream")
	return r
}

func TestOfflineGGUFHTTP(t *testing.T) {
	s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir())}
	for _, want := range []int{201, 409} {
		w := httptest.NewRecorder()
		requestBodyLimitMiddleware(http.HandlerFunc(s.handleGGUFImport)).ServeHTTP(w, offlineGGUFRequest(t))
		if w.Code != want {
			t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
		}
		if want == 201 {
			var receipt models.GGUFImportReceipt
			if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil || receipt.ModelID != "fixture" || len(receipt.SHA256) != 64 {
				t.Fatalf("receipt: %s", w.Body.String())
			}
		}
	}
}

type unreadableGGUFBody struct{ t *testing.T }

func (b unreadableGGUFBody) Read([]byte) (int, error) {
	b.t.Fatal("unauthorized body read")
	return 0, nil
}
func (b unreadableGGUFBody) Close() error { return nil }

func TestOfflineGGUFPermissionBeforeBody(t *testing.T) {
	store := users.NewUserStore("")
	_, key, err := store.CreateUser("reader", "password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{RequireAuth: true}, registry: models.NewRegistry(t.TempDir())}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	for _, token := range []string{"", key} {
		r := offlineGGUFRequest(t)
		r.Body = unreadableGGUFBody{t}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		auth.Wrap(http.HandlerFunc(s.handleGGUFImport)).ServeHTTP(w, r)
		want := 401
		if token != "" {
			want = 403
		}
		if w.Code != want {
			t.Fatalf("status %d want %d", w.Code, want)
		}
	}
}

func TestOfflineGGUFRejectedRequestDoesNotReadBody(t *testing.T) {
	s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir())}
	for _, test := range []struct {
		method, query, media string
		status               int
	}{
		{"GET", "?filename=x.gguf&size=1048576", "application/octet-stream", 405},
		{"POST", "?filename=x.gguf&size=34359738369", "application/octet-stream", 422},
		{"POST", "?filename=x.gguf&size=1048576", "text/plain", 422},
		{"POST", "?filename=../x.gguf&size=1048576", "application/octet-stream", 422},
	} {
		r := httptest.NewRequest(test.method, modelGGUFImportPath+test.query, nil)
		r.Header.Set("Content-Type", test.media)
		r.Body = unreadableGGUFBody{t}
		w := httptest.NewRecorder()
		s.handleGGUFImport(w, r)
		if w.Code != test.status {
			t.Fatalf("status %d want %d", w.Code, test.status)
		}
	}
}
