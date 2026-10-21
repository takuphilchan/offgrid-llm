package main

import (
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestModelOperationPollingCancellationRequestsSettlement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("missing auth")
		}
		if r.Method == "GET" {
			cancel()
			io.WriteString(w, `{"id":"op","state":"downloading"}`)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/api/v2/models/operations/op/cancel" {
			stopped = true
			io.WriteString(w, `{"state":"cancelling"}`)
			return
		}
		t.Error("unexpected endpoint")
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "fixture-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = waitForModelOperation(ctx, client, "op"); err != context.Canceled || !stopped {
		t.Fatal("cancellation did not request settlement", err, stopped)
	}
}

func TestModelListPreservesProviderMetadataThroughAuthenticatedService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/models" || r.URL.Query().Get("category") != "language" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Errorf("unexpected model inventory request: %s auth=%q", r.URL, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"models":[{"id":"offline-mlx","engine":"mlx","format":"mlx_safetensors","backend":"metal","readiness":{"installed":true,"runtime_ready":false,"qualified":false}}]}`)
	}))
	defer server.Close()
	t.Setenv("OFFGRID_SERVER_URL", server.URL)
	t.Setenv("OFFGRID_API_KEY", "fixture-key")
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = old; _ = read.Close(); _ = write.Close() }()
	if err := runModelPackageCommand(context.Background(), []string{"list", "--category", "language"}); err != nil {
		t.Fatal(err)
	}
	_ = write.Close()
	var result struct {
		Models []struct {
			Engine    string `json:"engine"`
			Format    string `json:"format"`
			Readiness struct {
				Qualified bool `json:"qualified"`
			} `json:"readiness"`
		} `json:"models"`
	}
	if err := json.NewDecoder(read).Decode(&result); err != nil || len(result.Models) != 1 || result.Models[0].Engine != "mlx" || result.Models[0].Format != "mlx_safetensors" || result.Models[0].Readiness.Qualified {
		t.Fatalf("provider metadata was lost or promoted: %#v err=%v", result, err)
	}
}
