package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestPublicSystemIdentityIsExactAndContainsNoPrivateState(t *testing.T) {
	uiDir := t.TempDir()
	index := []byte("<html>\n<script src=\"assets/new-build.js\"></script></html>\n")
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte(strings.ReplaceAll(string(index), "\n", "\r\n")), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_UI_DIR", uiDir)
	s := &Server{version: "test-version"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", s.handleSystemIdentity)
	mux.HandleFunc("/api/v2/system/private", func(w http.ResponseWriter, r *http.Request) { t.Error("authentication bypassed") })
	auth := users.NewMiddleware(nil)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(mux)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/v2/system", nil))
	var identity SystemIdentity
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &identity) != nil {
		t.Fatalf("bad identity: %s", response.Body.String())
	}
	digest := sha256.Sum256(index)
	if identity.Product != "offgrid" || identity.APIVersion != 2 || identity.Version != s.version || identity.UIBuildID != hex.EncodeToString(digest[:]) {
		t.Fatalf("wrong identity: %+v", identity)
	}
	if strings.Contains(response.Body.String(), uiDir) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private path or cached identity")
	}
	for _, path := range []string{"/api/v2/system/private", "/api/v2/system-extra"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("%s bypasses authentication: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.handleSystemIdentity(w, httptest.NewRequest("POST", "/api/v2/system", nil))
	if w.Code != 405 {
		t.Fatal("mutation accepted")
	}
}

func TestInjectedBuildRevision(t *testing.T) {
	previous := BuildRevision
	BuildRevision = "0123456789-test-build"
	t.Cleanup(func() { BuildRevision = previous })
	w := httptest.NewRecorder()
	(&Server{version: "test"}).handleSystemIdentity(w, httptest.NewRequest("GET", "/api/v2/system", nil))
	var identity SystemIdentity
	if err := json.Unmarshal(w.Body.Bytes(), &identity); err != nil {
		t.Fatal(err)
	}
	if identity.Revision != BuildRevision {
		t.Fatalf("container revision lost: %+v", identity)
	}
}

func TestDesktopBridgeIdentity(t *testing.T) {
	root := t.TempDir()
	index := []byte("<html>\ncontract renderer</html>\n")
	digest := sha256.Sum256(index)
	hash := hex.EncodeToString(digest[:])
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(strings.ReplaceAll(string(index), "\n", "\r\n")), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_UI_DIR", root)
	for _, tc := range []struct{ name, manifest, status string }{
		{"missing", "", "missing"},
		{"ready", `{"schema_version":1,"protocol":1,"ui_build_id":"` + hash + `"}`, "ready"},
		{"future protocol preserved for client assessment", `{"schema_version":1,"protocol":2,"ui_build_id":"` + hash + `"}`, "ready"},
		{"different renderer", `{"schema_version":1,"protocol":1,"ui_build_id":"` + strings.Repeat("b", 64) + `"}`, "invalid"},
		{"missing fields", `{}`, "invalid"},
		{"corrupt", `{`, "invalid"},
		{"schema", `{"schema_version":2,"protocol":1,"ui_build_id":"` + hash + `"}`, "invalid"},
		{"oversized", strings.Repeat(" ", 4097), "invalid"},
		{"null", `null`, "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(root, "desktop-compatibility.json"), []byte(tc.manifest), 0600); err != nil {
					t.Fatal(err)
				}
			}
			w := httptest.NewRecorder()
			(&Server{version: "test"}).handleSystemIdentity(w, httptest.NewRequest("GET", "/api/v2/system", nil))
			var identity SystemIdentity
			if err := json.Unmarshal(w.Body.Bytes(), &identity); err != nil {
				t.Fatal(err)
			}
			if identity.DesktopBridge == nil || identity.DesktopBridge.Status != tc.status {
				t.Fatalf("unexpected bridge: %+v", identity.DesktopBridge)
			}
			if strings.Contains(w.Body.String(), root) {
				t.Fatal("private path exposed")
			}
			if tc.status == "ready" && identity.DesktopBridge.UIBuildID != hash {
				t.Fatal("renderer fingerprint lost")
			}
		})
	}
	// A manifest in one root must not qualify a different custom renderer root.
	other := t.TempDir()
	if result := desktopBridgeIdentity(other, hash); result.Status != "missing" {
		t.Fatal(result)
	}
}
