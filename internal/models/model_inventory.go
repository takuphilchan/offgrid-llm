package models

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ModelCategory is a user-facing capability filter, never a runtime selector.
type ModelCategory string

const (
	CategoryLanguage    ModelCategory = "language"
	CategoryEmbeddings  ModelCategory = "embeddings"
	CategoryRecognition ModelCategory = "speech_recognition"
	CategoryGeneration  ModelCategory = "speech_generation"
)

func (c ModelCategory) Valid() bool {
	return slices.Contains([]ModelCategory{CategoryLanguage, CategoryEmbeddings, CategoryRecognition, CategoryGeneration}, c)
}

type LegacyFileTarget struct {
	ModelID    string `json:"model_id"`
	Repository string `json:"repository"`
	File       string `json:"file"`
	Revision   string `json:"revision"`
	SHA256     string `json:"sha256,omitempty"`
}

// InstallationTarget is an explicit union. Consumers must not infer the target
// type from a suffix or cast a package to a legacy filename.
type InstallationTarget struct {
	Kind    string                `json:"kind"`
	File    *LegacyFileTarget     `json:"file,omitempty"`
	Package *ModelPackageManifest `json:"package,omitempty"`
}

func (t InstallationTarget) Validate() error {
	switch t.Kind {
	case "legacy_file":
		if t.File == nil || t.Package != nil || t.File.ModelID == "" || !ValidHubRepository(t.File.Repository) || !validArtifactPath(t.File.File) || !strings.HasSuffix(strings.ToLower(t.File.File), ".gguf") || t.File.Revision == "" {
			return fmt.Errorf("invalid legacy file target")
		}
	case "package":
		if t.File != nil || t.Package == nil {
			return fmt.Errorf("invalid package target")
		}
		return t.Package.Validate()
	default:
		return fmt.Errorf("unsupported installation target")
	}
	return nil
}

type ModelReadiness struct {
	Installed         bool   `json:"installed"`
	Integrity         string `json:"integrity"`
	RuntimeCompatible bool   `json:"runtime_compatible"`
	SmokeTested       bool   `json:"smoke_tested"`
	Qualified         bool   `json:"qualified"`
	Issue             string `json:"issue,omitempty"`
}

type ModelProvenance struct {
	Kind       string `json:"kind"`
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
}

type TypedModelVariant struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	SizeBytes  int64              `json:"size_bytes"`
	Target     InstallationTarget `json:"target"`
	Provenance ModelProvenance    `json:"provenance"`
	Readiness  ModelReadiness     `json:"readiness"`
}

type TypedCatalogEntry struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Category      ModelCategory       `json:"category"`
	Capabilities  []ModelCapability   `json:"capabilities"`
	License       string              `json:"license"`
	Variants      []TypedModelVariant `json:"variants"`
	SourceNotices []string            `json:"source_notices,omitempty"`
}

type PackageCatalogEntry struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Variants      []ModelPackageManifest `json:"variants"`
	SourceNotices []string               `json:"source_notices,omitempty"`
}

func packageCategory(m ModelPackageManifest) ModelCategory {
	if slices.Contains(m.Capabilities, CapabilityTranscription) {
		return CategoryRecognition
	}
	return CategoryGeneration
}

// TypedEntries is the shared projection; Models stays the unchanged v1/CLI
// single-file catalog. No inference/runtime readiness is implied by catalog data.
func (c *ModelCatalog) TypedEntries(category ModelCategory) []TypedCatalogEntry {
	entries := []TypedCatalogEntry{}
	for _, e := range c.Models {
		cat, capabilities := CategoryLanguage, []ModelCapability{CapabilityChat}
		if e.Type == "embedding" || e.IsEmbeddingModel() {
			cat, capabilities = CategoryEmbeddings, []ModelCapability{CapabilityEmbeddings}
		} else if !api.IsChatModelType(e.Type) {
			continue
		}
		if category != "" && category != cat {
			continue
		}
		entry := TypedCatalogEntry{ID: e.ID, Name: e.Name, Description: e.Description, Category: cat, Capabilities: capabilities, License: e.License, Variants: []TypedModelVariant{}}
		for _, v := range e.Variants {
			for _, source := range v.Sources {
				u, err := url.Parse(source.URL)
				if err != nil || u.Scheme != "https" || u.Host != "huggingface.co" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
					continue
				}
				parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
				if len(parts) < 5 || parts[2] != "resolve" {
					continue
				}
				file := &LegacyFileTarget{ModelID: e.ID, Repository: parts[0] + "/" + parts[1], Revision: parts[3], File: strings.Join(parts[4:], "/"), SHA256: v.SHA256}
				target := InstallationTarget{Kind: "legacy_file", File: file}
				if target.Validate() != nil {
					continue
				}
				entry.Variants = append(entry.Variants, TypedModelVariant{ID: v.Quantization, Name: v.Quantization, SizeBytes: v.Size, Target: target, Provenance: ModelProvenance{Kind: "catalog_source", Repository: file.Repository, Revision: file.Revision}, Readiness: ModelReadiness{Integrity: "unchecked"}})
				break
			}
		}
		entries = append(entries, entry)
	}
	for _, e := range c.Packages {
		for _, m := range e.Variants {
			cat := packageCategory(m)
			if category != "" && category != cat {
				continue
			}
			var size int64
			for _, a := range m.Artifacts {
				size += a.Size
			}
			// Catalog packages use pinned, reviewed data, not a promise that the
			// runtime is installed, tested or suitable for this hardware.
			variant := TypedModelVariant{ID: m.Revision, Name: m.Name, SizeBytes: size, Target: InstallationTarget{Kind: "package", Package: &m}, Provenance: ModelProvenance{Kind: "curated_manifest", Revision: m.Revision}, Readiness: ModelReadiness{Integrity: "unchecked", Issue: "runtime_unavailable"}}
			found := false
			for i := range entries {
				if entries[i].ID == e.ID && entries[i].Category == cat {
					entries[i].Variants = append(entries[i].Variants, variant)
					found = true
					break
				}
			}
			if !found {
				entries = append(entries, TypedCatalogEntry{ID: e.ID, Name: e.Name, Description: e.Description, Category: cat, Capabilities: append([]ModelCapability{}, m.Capabilities...), License: m.License, SourceNotices: append([]string{}, e.SourceNotices...), Variants: []TypedModelVariant{variant}})
			}
		}
	}
	return entries
}

type InstalledModel struct {
	ID           string            `json:"id"`
	Revision     string            `json:"revision,omitempty"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Category     ModelCategory     `json:"category,omitempty"`
	Capabilities []ModelCapability `json:"capabilities"`
	Readiness    ModelReadiness    `json:"readiness"`
	Provenance   ModelProvenance   `json:"provenance"`
	Package      *PackageState     `json:"package,omitempty"`
	Legacy       *api.Model        `json:"legacy,omitempty"`
}

// Inventory preserves legacy IDs exactly, and includes corrupt packages as
// repairable records instead of quietly losing them from the installed list.
func Inventory(legacy []api.Model, packages []PackageState, category ModelCategory) []InstalledModel {
	result := []InstalledModel{}
	for _, m := range legacy {
		cat, caps := CategoryLanguage, []ModelCapability{CapabilityChat}
		if m.Type == "embedding" {
			cat, caps = CategoryEmbeddings, []ModelCapability{CapabilityEmbeddings}
		} else if !m.SupportsChat() {
			continue
		}
		if category != "" && category != cat {
			continue
		}
		result = append(result, InstalledModel{ID: m.ID, Name: m.ID, Kind: "legacy_file", Category: cat, Capabilities: caps, Legacy: &m, Readiness: ModelReadiness{Installed: true, Integrity: "unchecked"}, Provenance: ModelProvenance{Kind: "legacy"}})
	}
	for _, p := range packages {
		entry := InstalledModel{ID: p.ID, Revision: p.Revision, Name: p.ID, Kind: "package", Capabilities: []ModelCapability{}, Package: &p, Readiness: ModelReadiness{Installed: p.Installed, Integrity: p.Integrity, RuntimeCompatible: p.RuntimeCompatible, SmokeTested: p.SmokeTested, Qualified: p.Qualified, Issue: p.Issue}, Provenance: ModelProvenance{Kind: p.Provenance}}
		if p.Manifest != nil {
			entry.Name = p.Manifest.Name
			entry.Category = packageCategory(*p.Manifest)
			entry.Capabilities = append(entry.Capabilities, p.Manifest.Capabilities...)
		}
		if category != "" && category != entry.Category {
			continue
		}
		result = append(result, entry)
	}
	return result
}
