package models

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestTypedCatalogPreservesLegacyTargetsAndSeparatesSpeech(t *testing.T) {
	c := DefaultCatalog()
	before, _ := json.Marshal(c.Models)
	m := testPackageManifest("piper")
	c.Packages = []PackageCatalogEntry{{ID: m.ID, Name: m.Name, Variants: []ModelPackageManifest{m}}}
	entries := c.TypedEntries("")
	if len(entries) != len(c.Models)+1 {
		t.Fatalf("entries: %d", len(entries))
	}
	for i, original := range c.Models {
		entry := entries[i]
		if entry.ID != original.ID || len(entry.Variants) != len(original.Variants) {
			t.Fatalf("legacy projection changed: %s", original.ID)
		}
		for _, variant := range entry.Variants {
			if err := variant.Target.Validate(); err != nil {
				t.Fatal(err)
			}
			if variant.Target.Kind != "legacy_file" || variant.Target.File.ModelID != original.ID {
				t.Fatal("lost original ID")
			}
			if variant.Readiness.Installed || variant.Readiness.RuntimeCompatible || variant.Readiness.Qualified {
				t.Fatal("catalog invented readiness")
			}
		}
	}
	after, _ := json.Marshal(c.Models)
	if string(before) != string(after) {
		t.Fatal("v1 catalog mutated")
	}
	speech := c.TypedEntries(CategoryGeneration)
	if len(speech) != 1 || speech[0].Variants[0].Target.Package.ID != m.ID {
		t.Fatal("speech missing")
	}
	for _, entry := range c.TypedEntries(CategoryLanguage) {
		if entry.Category != CategoryLanguage || entry.Variants[0].Target.Kind != "legacy_file" {
			t.Fatal("speech/embeddings leaked into language")
		}
	}
	if len(c.TypedEntries(CategoryEmbeddings)) < 4 {
		t.Fatal("embedding choices lost")
	}
}

func TestInstallationTargetDiscriminator(t *testing.T) {
	m := testPackageManifest("whisper")
	f := &LegacyFileTarget{ModelID: "original-id", Repository: "owner/repo", File: "sub/model.gguf", Revision: "main"}
	for _, invalid := range []InstallationTarget{{}, {Kind: "package"}, {Kind: "legacy_file"}, {Kind: "package", Package: &m, File: f}, {Kind: "legacy_file", File: f, Package: &m}, {Kind: "script"}} {
		if invalid.Validate() == nil {
			t.Fatalf("accepted %+v", invalid)
		}
	}
	for _, valid := range []InstallationTarget{{Kind: "package", Package: &m}, {Kind: "legacy_file", File: f}} {
		if err := valid.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInventoryKeepsIndependentEvidenceAndCorruptPackages(t *testing.T) {
	m := testPackageManifest("zipformer-streaming")
	items := Inventory([]api.Model{{ID: "unchanged-llm"}, {ID: "unchanged-embedding", Type: "embedding"}, {ID: "never-chat", Type: "speech_synthesis"}}, []PackageState{{ID: m.ID, Revision: m.Revision, Manifest: &m, Installed: true, Integrity: "checked", Issue: "runtime_unavailable", Provenance: "local_untrusted"}, {ID: "broken", Revision: "rev1", Installed: true, Integrity: "failed", Issue: "manifest_invalid"}}, "")
	if len(items) != 4 || items[0].ID != "unchanged-llm" || items[1].ID != "unchanged-embedding" {
		t.Fatalf("inventory: %+v", items)
	}
	if items[2].Category != CategoryRecognition || items[2].Readiness.RuntimeCompatible || items[2].Readiness.SmokeTested || items[2].Readiness.Qualified || items[2].Provenance.Kind != "local_untrusted" {
		t.Fatal("integrity promoted to readiness")
	}
	if items[3].ID != "broken" || items[3].Package == nil {
		t.Fatal("corrupt model disappeared")
	}
}
