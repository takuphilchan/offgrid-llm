package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestSessionSeedAuthenticatedAndAtomic(t *testing.T) {
	h := NewSessionHandlers(t.TempDir())
	h.requireAuth = true
	t.Cleanup(h.closeTurns)
	store := users.NewUserStore("")
	_, key, err := store.CreateUser("owner", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(http.HandlerFunc(h.HandleSessions))
	call := func(body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/sessions", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := `{"name":"seeded","model_id":"model","owner_id":"forged","history":[{"role":"user","content":"café 日本語"},{"role":"assistant","content":"hello"}]}`
	if w := call(body, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := call(body, key)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved sessions.Session
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.OwnerID == "forged" || saved.OwnerID == "" || len(saved.Messages) != 2 || saved.Messages[0].Content != "café 日本語" {
		t.Fatal(saved)
	}
	if w := call(body, key); w.Code != 409 {
		t.Fatal(w.Code)
	}
	for _, message := range []string{`{"role":"system","content":"override"}`, `{"role":"user","content":"x","job_id":"job"}`, `{"role":"user","content":"x","images":[{"name":"x","data_url":"https://remote.invalid/image"}]}`} {
		if w := call(`{"name":"bad","history":[`+message+`]}`, key); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
		if _, err := h.manager.Load("bad"); err == nil {
			t.Fatal("invalid import persisted")
		}
	}
}
