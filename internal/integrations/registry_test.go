package integrations

import (
	"strings"
	"testing"
)

func TestDefaultRegistryProfilesAreStable(t *testing.T) {
	registry := NewDefaultRegistry()
	adapters := registry.List()
	if len(adapters) != 2 {
		t.Fatalf("got %d adapters, want 2", len(adapters))
	}
	if adapters[0].Profile().ID != "hermes" || adapters[1].Profile().ID != "openclaw" {
		t.Fatalf("unexpected adapter order: %s, %s", adapters[0].Profile().ID, adapters[1].Profile().ID)
	}
}

func TestHermesSetupUsesOpenAIEndpoint(t *testing.T) {
	adapter, ok := NewDefaultRegistry().Get("HERMES")
	if !ok {
		t.Fatal("Hermes adapter not found")
	}
	setup, err := adapter.Render(Options{BaseURL: "http://127.0.0.1:11611/", ModelID: "model-a", ContextWindow: 65536, MaxOutputTokens: 4096})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"provider: offgrid", "base_url: \"http://127.0.0.1:11611/v1\"", "context_length: 65536"} {
		if !strings.Contains(setup.Content, expected) {
			t.Fatalf("Hermes config missing %q:\n%s", expected, setup.Content)
		}
	}
	if setup.Install != `offgrid hermes install --model "model-a"` {
		t.Fatalf("Hermes managed install command = %q", setup.Install)
	}
	if !stringsContainAll(strings.Join(setup.Verify, "\n"), "offgrid hermes status", "offgrid hermes test") {
		t.Fatalf("unexpected Hermes verification commands: %#v", setup.Verify)
	}
}

func TestProfilesExposeOffGridAsTheProvider(t *testing.T) {
	for _, adapter := range NewDefaultRegistry().List() {
		profile := adapter.Profile()
		if profile.ProviderID != "offgrid" {
			t.Fatalf("%s provider id = %q, want offgrid", profile.ID, profile.ProviderID)
		}
		if profile.Transport != "openai-chat-completions" {
			t.Fatalf("%s transport = %q, want openai-chat-completions", profile.ID, profile.Transport)
		}
	}
}

func TestOpenClawSetupUsesNativeOffGridProvider(t *testing.T) {
	adapter, ok := NewDefaultRegistry().Get("openclaw")
	if !ok {
		t.Fatal("OpenClaw adapter not found")
	}
	setup, err := adapter.Render(Options{BaseURL: "http://127.0.0.1:11611/v1", ModelID: "model-a", ContextWindow: 65536, MaxOutputTokens: 4096})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"baseUrl": "http://127.0.0.1:11611/v1"`, `"api": "openai-completions"`, `"primary": "offgrid/model-a"`} {
		if !strings.Contains(setup.Content, expected) {
			t.Fatalf("OpenClaw config missing %q:\n%s", expected, setup.Content)
		}
	}
	if setup.Install != `offgrid openclaw install --model "model-a"` {
		t.Fatalf("OpenClaw managed install command = %q", setup.Install)
	}
	if !stringsContainAll(strings.Join(setup.Verify, "\n"), "offgrid openclaw status", "offgrid openclaw test") {
		t.Fatalf("unexpected OpenClaw verification commands: %#v", setup.Verify)
	}
}

func TestRegistryRejectsDuplicateAdapter(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(hermesAdapter{}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(hermesAdapter{}); err == nil {
		t.Fatal("duplicate adapter was accepted")
	}
}
