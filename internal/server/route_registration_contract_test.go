package server

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateRouteFixture = flag.Bool("update-route-fixture", false, "replace the frozen server route manifest before a mechanical move")

// This intentionally compares source-level registration expressions. Runtime
// HTTP fixtures separately cover auth, body limits, errors, and streaming.
func TestServerRouteRegistrationContract(t *testing.T) {
	actual, err := serverRouteManifest()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("testdata", "route-registration.golden")
	if *updateRouteFixture {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, []byte(actual), 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(expected) {
		t.Fatalf("route or outer middleware registration drifted from frozen fixture; review path, handler, permission, rate and order before updating %s", fixture)
	}
}

func serverRouteManifest() (string, error) {
	var routes, middleware []string
	var routeCalls, routeDefinitions []string
	for _, name := range []string{"server.go", "router.go"} {
		data, err := os.ReadFile(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "s.register") && strings.HasSuffix(line, "(mux, guards)") {
				routeCalls = append(routeCalls, strings.TrimPrefix(strings.SplitN(line, "(", 2)[0], "s."))
			}
			if strings.HasPrefix(line, "func (s *Server) register") {
				fields := strings.Fields(line)
				routeDefinitions = append(routeDefinitions, strings.SplitN(fields[3], "(", 2)[0])
			}
			if strings.HasPrefix(line, "mux.Handle(") || strings.HasPrefix(line, "mux.HandleFunc(") {
				if cut := strings.Index(line, " //"); cut >= 0 {
					line = strings.TrimSpace(line[:cut])
				}
				routes = append(routes, line)
			}
			if strings.HasPrefix(line, "handler = ") &&
				(strings.Contains(line, "authMiddleware.Wrap(") ||
					strings.Contains(line, "browserTransport(") ||
					strings.Contains(line, "requestBodyLimitMiddleware(") ||
					strings.Contains(line, "loggingMiddleware(")) {
				if cut := strings.Index(line, " //"); cut >= 0 {
					line = strings.TrimSpace(line[:cut])
				}
				middleware = append(middleware, line)
			}
		}
	}
	if len(routeCalls) != 23 || len(routeDefinitions) != len(routeCalls) {
		return "", fmt.Errorf("unexpected route group call/definition count: %d/%d", len(routeCalls), len(routeDefinitions))
	}
	for i := range routeCalls {
		if routeCalls[i] != routeDefinitions[i] {
			return "", fmt.Errorf("route group runtime call order differs from source registration order at %d: %s/%s", i, routeCalls[i], routeDefinitions[i])
		}
	}
	if len(routes) != 137 || len(middleware) != 4 {
		return "", fmt.Errorf("unexpected baseline manifest shape: %d routes, %d outer middleware", len(routes), len(middleware))
	}
	return "# ordered route registrations\n" + strings.Join(routes, "\n") +
		"\n# outer middleware wrapping order\n" + strings.Join(middleware, "\n") + "\n", nil
}

// These isolated HTTP checks complement the route fixture. They do not prove
// native desktop behavior or a live listener's network exposure.
func TestServerHTTPBaseline(t *testing.T) {
	s := newTestServer(t)
	t.Run("router registration and health", func(t *testing.T) {
		w := httptest.NewRecorder()
		s.buildRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("router health response: status=%d, content-type=%q", w.Code, w.Header().Get("Content-Type"))
		}
	})
	t.Run("health", func(t *testing.T) {
		w := httptest.NewRecorder()
		s.handleHealth(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" || !strings.Contains(w.Body.String(), `"status":"healthy"`) {
			t.Fatalf("health response: status=%d, content-type=%q, body=%s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
	})
	t.Run("error payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeErrorWithCode(w, "fixture failure", http.StatusBadRequest, "fixture_code")
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"fixture_code"`) || !strings.Contains(w.Body.String(), `"fixture failure"`) {
			t.Fatalf("error response: status=%d, body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("SSE headers and initial flush", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/loading/progress/stream", nil).WithContext(ctx))
		if w.Header().Get("Content-Type") != "text/event-stream" || w.Header().Get("Cache-Control") != "no-cache" || !w.Flushed || !strings.HasPrefix(w.Body.String(), "data: ") {
			t.Fatalf("SSE response: content-type=%q cache=%q flushed=%v body=%q", w.Header().Get("Content-Type"), w.Header().Get("Cache-Control"), w.Flushed, w.Body.String())
		}
	})
}
