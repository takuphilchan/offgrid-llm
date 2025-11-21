package models

import "testing"

func TestResolveProjectorSourcePrefersCompanion(t *testing.T) {
	files := []HFFile{
		{Filename: "vlm-r1-qwen2.5vl-3b-ovd-i1.gguf"},
		{Filename: "vlm-r1-qwen2.5vl-3b-ovd-i1.mmproj.gguf", Size: 1234},
	}

	source, err := (*HuggingFaceClient)(nil).ResolveProjectorSource("mradermacher/VLM-R1-Qwen2.5VL-3B-OVD-0321-i1-GGUF", files, "vlm-r1-qwen2.5vl-3b-ovd-i1.gguf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source == nil {
		t.Fatalf("expected projector source, got nil")
	}
	if source.Source != "companion" {
		t.Fatalf("expected companion source, got %s", source.Source)
	}
	if source.ModelID != "mradermacher/VLM-R1-Qwen2.5VL-3B-OVD-0321-i1-GGUF" {
		t.Fatalf("unexpected model ID: %s", source.ModelID)
	}
	if source.File.Filename != "vlm-r1-qwen2.5vl-3b-ovd-i1.mmproj.gguf" {
		t.Fatalf("unexpected filename: %s", source.File.Filename)
	}
}

func TestResolveProjectorSourceFallsBackWhenMissing(t *testing.T) {
	source, err := (*HuggingFaceClient)(nil).ResolveProjectorSource("mradermacher/VLM-R1-Qwen2.5VL-3B-OVD-0321-i1-GGUF", nil, "vlm-r1-qwen2.5vl-3b-ovd-i1.gguf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source == nil {
		t.Fatalf("expected fallback projector source, got nil")
	}
	if source.Source != "fallback" {
		t.Fatalf("expected fallback source, got %s", source.Source)
	}
	if source.ModelID != "koboldcpp/mmproj" {
		t.Fatalf("unexpected fallback repo: %s", source.ModelID)
	}
	if source.File.Filename != "Qwen2.5-Omni-3B-mmproj-Q8_0.gguf" {
		t.Fatalf("unexpected fallback filename: %s", source.File.Filename)
	}
	if source.Reason == "" {
		t.Fatalf("expected fallback reason to be populated")
	}
}
