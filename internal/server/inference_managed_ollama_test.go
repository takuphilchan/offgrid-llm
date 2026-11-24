package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestManagedOllamaFileEnrollmentUsesExistingSelectorWithoutLaunching(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "endpoints.json")
	entry := endpointEnrollmentFixture()
	entry.URL, entry.AllowedIPs, entry.CredentialEnv, entry.Model = "http://127.0.0.1:19991", []string{"127.0.0.1"}, "", "fixture:tag"
	entry.ManagedOllama = &inference.ManagedOllamaConfig{RuntimeDirectory: filepath.Join(directory, "runtime"), Executable: "ollama.exe", RuntimeFiles: map[string]string{"ollama.exe": "sha256:" + strings.Repeat("a", 64)}, ModelDirectory: filepath.Join(directory, "models"), StateDirectory: filepath.Join(directory, "state"), TrustedSingleUserHost: true, AllowUnauthenticatedLoopback: true}
	if runtime.GOOS == "linux" {
		entry.ManagedOllama.NamespaceLauncher = "/usr/bin/unshare"
		entry.ManagedOllama.NamespaceLauncherSHA256 = "sha256:" + strings.Repeat("b", 64)
	}
	writeEndpointEnrollment(t, path, entry)
	registry := inference.NewProviderRegistry()
	s := &Server{config: &config.Config{InferenceEndpointsFile: path, DataDir: t.TempDir()}, providerRegistry: registry}
	err := s.InstallConfiguredHTTPEndpoints()
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		if err == nil {
			t.Fatal("unsupported platform admitted owned runtime")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	reviewEndpointFixture(t, s, entry)
	descriptor, selected, err := s.selectNativeChat(context.Background(), "endpoint:lab@r1")
	if err != nil || !selected || descriptor.Backend != "local-ollama" {
		t.Fatal(descriptor, selected, err)
	}
	models := s.listedEndpointModels(context.Background())
	if len(models) != 1 || models[0].Backend != "local-ollama" || models[0].RuntimeReady || models[0].Qualified || models[0].SmokeTested {
		t.Fatal("enrollment claimed unevidenced readiness", models)
	}
	// Installation is metadata-only. The registered real factory is reachable,
	// but absent pinned state fails locally before any process or network call.
	if provider, err := registry.Open(context.Background(), descriptor); err == nil || provider != nil {
		t.Fatal("missing managed runtime was accepted")
	}
	entry.ManagedOllama.AllowUnauthenticatedLoopback = false
	writeEndpointEnrollment(t, path, entry)
	if _, _, err := s.selectNativeChat(context.Background(), "endpoint:lab@r1"); err == nil {
		t.Fatal("changed consent retained stale selection")
	}
	if provider, err := registry.Open(context.Background(), descriptor); err == nil || provider != nil {
		t.Fatal("revoked managed enrollment reached factory")
	}
}

func TestManagedOllamaEnrollmentRejectsCredentialAndMultipleActors(t *testing.T) {
	for _, actors := range [][]string{{"local-admin"}, {"local-admin", "another"}} {
		path := filepath.Join(t.TempDir(), "endpoints.json")
		entry := endpointEnrollmentFixture()
		entry.Actors = actors
		entry.ManagedOllama = &inference.ManagedOllamaConfig{}
		if len(actors) > 1 {
			entry.CredentialEnv = ""
		}
		writeEndpointEnrollment(t, path, entry)
		if _, err := readInferenceEndpoints(path); err == nil {
			t.Fatal("managed runtime accepted ambiguous actor or upstream credential")
		}
	}
}
