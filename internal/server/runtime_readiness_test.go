package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestRuntimePreparationAuthority(t *testing.T) {
	t.Setenv("OFFGRID_BIN_DIR", t.TempDir())
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", filepath.Join(t.TempDir(), "missing"))
	store := users.NewUserStore("")
	_, reader, err := store.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, admin, err := store.CreateUser("admin", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	cache := inference.NewModelCache(1, 0, t.TempDir())
	defer cache.StopMonitor()
	s := &Server{config: &config.Config{RequireAuth: true}, modelCache: cache}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(http.HandlerFunc(s.handleRuntimePreparation))
	for _, tc := range []struct {
		method, token string
		code          int
	}{
		{"GET", "", 401}, {"POST", "", 401}, {"GET", reader, 200}, {"POST", reader, 403}, {"POST", admin, 503}, {"DELETE", admin, 405},
	} {
		r := httptest.NewRequest(tc.method, "/api/v2/runtime", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s: got %d want %d: %s", tc.method, w.Code, tc.code, w.Body.String())
		}
	}
}
