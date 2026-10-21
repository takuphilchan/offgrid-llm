package models

import "testing"

func TestAdapterRegistryRejectsRemoteCodeAndUnknownEngine(t *testing.T) {
	base := ModelAdapterSpec{ID: "qwen", Revision: "adapter-1", Engine: EngineTransformers, Formats: []string{FormatSafetensors}, Modalities: []string{"text"}, ToolCalling: "unqualified", Template: "chatml", Tokenization: "template_exact_ids", Streaming: "text_delta_v1", ToolEnvelope: "none"}
	r := NewAdapterRegistry()
	if err := r.Register(base); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Resolve("qwen", "adapter-1"); !ok {
		t.Fatal("registered adapter not resolvable")
	}
	remote := base
	remote.ID = "remote"
	remote.RemoteCodeAllowed = true
	if err := r.Register(remote); err == nil {
		t.Fatal("remote-code adapter accepted")
	}
	unknown := base
	unknown.ID = "unknown"
	unknown.Engine = "python"
	if err := r.Register(unknown); err == nil {
		t.Fatal("unknown engine accepted")
	}
	for _, test := range []struct {
		name   string
		modify func(*ModelAdapterSpec)
	}{
		{"implicit tokenizer", func(s *ModelAdapterSpec) { s.Tokenization = "" }},
		{"alternate tokenizer", func(s *ModelAdapterSpec) { s.Tokenization = "retokenize" }},
		{"alternate stream", func(s *ModelAdapterSpec) { s.Streaming = "raw_sse" }},
		{"unqualified tool envelope", func(s *ModelAdapterSpec) { s.ToolEnvelope = "json_calls" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			unsupported := base
			unsupported.ID = "unsupported"
			test.modify(&unsupported)
			if err := r.Register(unsupported); err == nil {
				t.Fatal("unimplemented adapter behavior was accepted")
			}
		})
	}
}

func TestAdapterRegistryMatchesOnlyPinnedCompatibleTextPackage(t *testing.T) {
	m := testPackageManifest("transformers-text")
	r := NewAdapterRegistry()
	if _, err := r.MatchPackage(m); err == nil {
		t.Fatal("unknown model family became executable")
	}
	spec := ModelAdapterSpec{ID: m.ModelFamily, Revision: m.AdapterRevision, Engine: EngineTransformers, Formats: []string{FormatSafetensors}, Modalities: []string{"text"}, ToolCalling: "unqualified", Template: "chatml", Tokenization: "template_exact_ids", Streaming: "text_delta_v1", ToolEnvelope: "none", MaxQualifiedContext: m.ContextLength}
	if err := r.Register(spec); err != nil {
		t.Fatal(err)
	}
	if _, err := r.MatchPackage(m); err != nil {
		t.Fatal(err)
	}
	m.ContextLength++
	if _, err := r.MatchPackage(m); err == nil {
		t.Fatal("context exceeding reviewed adapter was admitted")
	}
	m.ContextLength--
	m.Engine = EngineMLX
	if _, err := r.MatchPackage(m); err == nil {
		t.Fatal("mismatched provider was admitted")
	}
}
