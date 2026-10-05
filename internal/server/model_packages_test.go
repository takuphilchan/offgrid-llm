package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func packageUpload(t *testing.T, corrupt, extra bool) *http.Request {
	t.Helper()
	data := []byte("synthetic weights")
	digest := sha256.Sum256(data)
	m := models.ModelPackageManifest{SchemaVersion: 1, ID: "fixture", Revision: "rev-1", Name: "Test only", Architecture: "whisper", Runtime: models.PackageRuntime{Adapter: "whisper.cpp", Revision: "test-1"}, Capabilities: []models.ModelCapability{models.CapabilityTranscription}, Languages: []string{"en"}, SampleRates: []int{16000}, License: "CC0-1.0", Artifacts: []models.PackageArtifact{{Path: "weights.bin", Role: "weights", Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), License: "CC0-1.0"}}}
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, _ := writer.CreateFormField("manifest")
	if err := json.NewEncoder(part).Encode(m); err != nil {
		t.Fatal(err)
	}
	part, _ = writer.CreateFormFile("weights.bin", "weights.bin")
	if corrupt {
		data[0] = 'X'
	}
	part.Write(data)
	if extra {
		part, _ = writer.CreateFormFile("extra.py", "extra.py")
		part.Write([]byte("not executable"))
	}
	writer.Close()
	r := httptest.NewRequest(http.MethodPost, modelPackagePath, &b)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r
}

func TestModelPackageHTTPTransaction(t *testing.T) {
	for _, mode := range []string{"success", "corrupt", "extra"} {
		t.Run(mode, func(t *testing.T) {
			s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir())}
			w := httptest.NewRecorder()
			requestBodyLimitMiddleware(http.HandlerFunc(s.handleModelPackages)).ServeHTTP(w, packageUpload(t, mode == "corrupt", mode == "extra"))
			want := http.StatusCreated
			if mode != "success" {
				want = 422
			}
			if w.Code != want {
				t.Fatalf("import %d: %s", w.Code, w.Body.String())
			}
			items, err := s.registry.Packages().List()
			if err != nil {
				t.Fatal(err)
			}
			if mode != "success" {
				if len(items) != 0 {
					t.Fatal("invalid upload activated")
				}
				return
			}
			if len(items) != 1 || items[0].RuntimeCompatible || items[0].Qualified {
				t.Fatalf("inventory: %+v", items)
			}
			for _, operation := range []string{"verify", "remove"} {
				w = httptest.NewRecorder()
				s.handleModelPackages(w, httptest.NewRequest("POST", modelPackagePath+"/fixture/rev-1/"+operation, nil))
				if w.Code != 200 {
					t.Fatalf("%s: %d %s", operation, w.Code, w.Body.String())
				}
			}
		})
	}
}

func TestModelPackageHTTPAuthorization(t *testing.T) {
	store := users.NewUserStore("")
	_, userKey, err := store.CreateUser("reader", "password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, adminKey, err := store.CreateUser("manager", "password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{RequireAuth: true}, registry: models.NewRegistry(t.TempDir())}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(http.HandlerFunc(s.handleModelPackages))
	for _, c := range []struct {
		key    string
		status int
	}{{"", 401}, {userKey, 403}, {adminKey, 201}} {
		r := packageUpload(t, false, false)
		if c.key != "" {
			r.Header.Set("Authorization", "Bearer "+c.key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("status %d want %d: %s", w.Code, c.status, w.Body.String())
		}
	}
}

func TestSpeechPackageCannotReachChatInference(t *testing.T) {
	s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir())}
	w := httptest.NewRecorder()
	s.handleModelPackages(w, packageUpload(t, false, false))
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.handleChatCompletions(w, httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(`{"model":"fixture","messages":[{"role":"user","content":"hello"}]}`)))
	if w.Code != 404 {
		t.Fatalf("speech reached inference: %d %s", w.Code, w.Body.String())
	}
}

func TestPackageBodyLimitRetainsOrdinaryRequestLimit(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", modelPackagePath} {
		var limit int64
		handler := requestBodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.Copy(io.Discard, r.Body)
			var exceeded *http.MaxBytesError
			if errors.As(err, &exceeded) {
				limit = exceeded.Limit
			}
		}))
		reader := &zeroPackageReader{remaining: maxRequestBodyBytes + 1}
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", path, reader))
		if path == modelPackagePath && limit != 0 {
			t.Fatal("package incorrectly limited to JSON request size")
		}
		if path != modelPackagePath && limit != maxRequestBodyBytes {
			t.Fatalf("ordinary request limit changed: %d", limit)
		}
	}
}

type zeroPackageReader struct{ remaining int64 }

func (r *zeroPackageReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	clear(p[:n])
	r.remaining -= n
	return int(n), nil
}
