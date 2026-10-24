package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSharedClientAuthenticatesDiscoveryAndRejectsRedirects(t *testing.T) {
	redirect := false
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing scoped authentication")
		}
		if redirect {
			w.Header().Set("Location", "http://example.invalid")
			w.WriteHeader(302)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/setup") {
			if r.URL.Query().Get("model") != "downloaded/model" {
				t.Error("model ID changed")
			}
			fmt.Fprint(w, `{"integration":{"id":"fixture-third","ready":true},"setup":{}}`)
		} else {
			fmt.Fprint(w, `{"data":[],"integrations":[]}`)
		}
	}))
	defer server.Close()
	c, err := NewServiceClient(server.URL, "test-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.List(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Setup(context.Background(), "fixture-third", "downloaded/model", server.URL); err != nil {
		t.Fatal(err)
	}
	redirect = true
	if _, err = c.List(context.Background(), ""); err == nil {
		t.Fatal("redirect followed")
	}
	if requests != 4 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestSharedOptionsPreserveTaskAndRejectMissingValues(t *testing.T) {
	o, rest, err := ParseHostOptions([]string{"--model", "downloaded-model", "hello", "--server", "http://localhost:1234"}, HostOptions{}, true)
	if err != nil || o.ModelID != "downloaded-model" || len(rest) != 1 || rest[0] != "hello" {
		t.Fatalf("options=%+v rest=%v err=%v", o, rest, err)
	}
	if _, _, err = ParseHostOptions([]string{"--model", "--yes"}, HostOptions{}, false); err == nil {
		t.Fatal("missing value accepted")
	}
	for _, args := range [][]string{{"--model=downloaded-model", "hello"}, {"hello", "-m", "downloaded-model"}} {
		o, rest, err = ParseHostOptions(args, HostOptions{}, true)
		if err != nil || o.ModelID != "downloaded-model" || len(rest) != 1 || rest[0] != "hello" {
			t.Fatalf("options=%+v rest=%v err=%v", o, rest, err)
		}
	}
}
