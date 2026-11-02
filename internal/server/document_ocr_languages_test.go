package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestOCRLanguageAPIAuthorityAndOfflineImport(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e}
	store := users.NewUserStore("")
	_, admin, err := store.CreateUser("admin", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, reader, err := store.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return users.RequirePermission(users.PermissionRAGManage)(next).ServeHTTP
	}
	mux := http.NewServeMux()
	s.registerKnowledgeRoutes(mux, routeGuards{ragManagerOnly: guard, ragOnly: guard})
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(mux)
	request := func(token, method, language string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/v1/documents/ocr-languages?language="+language, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/octet-stream")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, token := range []string{"", reader} {
		if w := request(token, "POST", "eng", []byte("forged")); w.Code != 401 && w.Code != 403 {
			t.Fatal("unauthorized pack import", w.Code)
		}
	}
	if w := request(admin, "POST", "eng", []byte("forged")); w.Code != 422 || !strings.Contains(w.Body.String(), "document_ocr_language_invalid") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(admin, "GET", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"missing"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	path := os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
	if path == "" {
		t.Log("real offline language import is exercised only with its explicit fixture")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(admin, "POST", "eng", data); w.Code != 200 || !strings.Contains(w.Body.String(), "installed_verified") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(admin, "GET", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "installed_verified") {
		t.Fatal("pack not durable", w.Code, w.Body.String())
	}
	service := httptest.NewServer(handler)
	defer service.Close()
	client, err := serviceclient.New(service.URL, admin, service.Client())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"ocr-languages"}, {"import-ocr-language", "eng", path}} {
		if _, err = cli.RunKnowledgeService(context.Background(), client, args, nil, io.Discard, true); err != nil {
			t.Fatal("authenticated CLI parity", err)
		}
	}
}
