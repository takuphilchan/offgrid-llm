package main

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"io"
	"net/http"
	"net/http/httptest"
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
