package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

type doctorRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn doctorRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDoctorObservationNeverWritesModelStorage(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	if err := os.Mkdir(modelsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "sample.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: doctorRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header), Request: request}, nil
	})}
	detect := func() (*resource.SystemResources, error) {
		return &resource.SystemResources{OS: "test", Arch: "test", CPUCores: 4, TotalRAM: 4096}, nil
	}
	cfg := &config.Config{ModelsDir: modelsDir, ServerPort: 11611}
	snapshot, err := observeDoctor(context.Background(), cfg, detect, client, filepath.Join(root, "config.json"))
	if err != nil || !snapshot.StoragePresent || snapshot.ModelCount != 1 || !snapshot.NetworkOnline || !snapshot.ServerOnline {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	entries, err := os.ReadDir(modelsDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "sample.gguf" {
		t.Fatalf("doctor changed model storage: entries=%v err=%v", entries, err)
	}
	cfg.ModelsDir = filepath.Join(root, "missing-models")
	snapshot, err = observeDoctor(context.Background(), cfg, detect, client, filepath.Join(root, "config.json"))
	if err != nil || snapshot.StoragePresent {
		t.Fatalf("missing storage: snapshot=%+v err=%v", snapshot, err)
	}
	if _, err := os.Stat(cfg.ModelsDir); !os.IsNotExist(err) {
		t.Fatalf("doctor created missing storage: %v", err)
	}
}
