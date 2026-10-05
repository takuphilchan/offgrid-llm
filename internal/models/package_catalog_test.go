package models

import (
	"strings"
	"testing"
)

func TestCuratedSpeechCatalogHasCompletePinnedRealPackages(t *testing.T) {
	items, err := CuratedPackageResolutions()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("want four recipes, got %d", len(items))
	}
	seen := map[string]bool{}
	for _, r := range items {
		m := r.Manifest
		if seen[m.Architecture] {
			t.Fatal("duplicate recipe")
		}
		seen[m.Architecture] = true
		if !sourceRevision.MatchString(m.Runtime.Revision) || !sourceRevision.MatchString(m.Revision) || strings.Contains(m.ID, "fixture") {
			t.Fatal("placeholder runtime/source")
		}
		var total int64
		for _, a := range m.Artifacts {
			total += a.Size
			if a.Source == nil || a.Source.Revision != m.Revision || !strings.Contains(a.Source.Repository, r.Provenance.Repository) || a.License == "" {
				t.Fatal("unpinned artifact", a.Path)
			}
		}
		if total != r.TransferBytes || total < 1<<20 {
			t.Fatal("placeholder artifact data")
		}
		if m.Architecture == "kokoro" && len(m.Artifacts) < 350 {
			t.Fatal("phonemizer data omitted")
		}
		if m.Architecture == "piper" && len(r.SourceNotices) < 2 {
			t.Fatal("voice license hidden")
		}
	}
	cat := DefaultCatalog()
	if len(cat.TypedEntries(CategoryRecognition)) != 2 || len(cat.TypedEntries(CategoryGeneration)) != 2 {
		t.Fatal("speech is not in shared catalog")
	}
	for _, e := range cat.Models {
		if e.Type != "llm" && e.Type != "embedding" {
			t.Fatal("v1 projection contaminated")
		}
	}
	for _, e := range cat.TypedEntries(CategoryGeneration) {
		for _, v := range e.Variants {
			if v.Readiness.Qualified || v.Readiness.RuntimeCompatible {
				t.Fatal("catalog claims runtime readiness")
			}
		}
	}
}
