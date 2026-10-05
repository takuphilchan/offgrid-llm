package models

import (
	"embed"
	"encoding/json"
	"fmt"
)

// These are data-only manifests reviewed against immutable public repository
// metadata. They install neither runtime workers nor speech qualification.
//
//go:embed catalog-packages/*.json
var packageCatalogFiles embed.FS

func CuratedPackageResolutions() ([]PackageResolution, error) {
	files, err := packageCatalogFiles.ReadDir("catalog-packages")
	if err != nil {
		return nil, err
	}
	result := make([]PackageResolution, 0, len(files))
	for _, file := range files {
		data, err := packageCatalogFiles.ReadFile("catalog-packages/" + file.Name())
		if err != nil {
			return nil, err
		}
		var r PackageResolution
		if err = json.Unmarshal(data, &r); err != nil {
			return nil, err
		}
		if err = ValidateAcquisitionManifest(r.Manifest); err != nil {
			return nil, fmt.Errorf("invalid built-in package %s: %w", file.Name(), err)
		}
		r.Provenance.Kind = "curated_manifest"
		result = append(result, r)
	}
	return result, nil
}

func curatedPackageEntries() []PackageCatalogEntry {
	resolutions, err := CuratedPackageResolutions()
	if err != nil {
		panic(err)
	} // Immutable build assets; covered by catalog tests.
	entries := make([]PackageCatalogEntry, 0, len(resolutions))
	for _, r := range resolutions {
		m := r.Manifest
		entries = append(entries, PackageCatalogEntry{ID: m.ID, Name: m.Name, Description: "Complete speech model package. Speech runtime and qualification are not included.", Variants: []ModelPackageManifest{m}, SourceNotices: r.SourceNotices})
	}
	return entries
}
